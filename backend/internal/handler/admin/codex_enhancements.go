package admin

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// All endpoints retain the existing administrator authentication, audit,
// compliance and account-scoped-key middleware. Diagnostics additionally
// require the signed-in administrator's own normal gateway API key.
func (h *AccountHandler) SetCodexDiagnosticRouter(router http.Handler, keys *service.APIKeyService) {
	h.codexDiagnosticRouter = router
	h.codexDiagnosticKeys = keys
}

func (h *AccountHandler) CodexEnhancementOptions(c *gin.Context) {
	models, err := service.ModelTraceGPTModels()
	if err != nil {
		response.Error(c, 503, "Fingerprint bank unavailable")
		return
	}
	response.Success(c, gin.H{"timezones": service.OpenAIRequestTimezoneOptions(), "default_timezone": service.DefaultOpenAIRequestTimezone, "models": models, "bank_commit": service.ModelTraceBankCommit()})
}

func (h *AccountHandler) DetectRequestTimezone(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	api, ok := h.adminService.(interface {
		DetectOpenAIRequestTimezone(context.Context, int64) (*service.Account, error)
	})
	if !ok {
		response.Error(c, 503, "Timezone detection unavailable")
		return
	}
	account, err := api.DetectOpenAIRequestTimezone(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

var activeCodexDiagnostics sync.Map

type boundedDiagnosticWriter struct {
	header   http.Header
	body     bytes.Buffer
	status   int
	exceeded bool
	cancel   context.CancelFunc
}

func (w *boundedDiagnosticWriter) Header() http.Header { return w.header }
func (w *boundedDiagnosticWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}
func (w *boundedDiagnosticWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	if w.body.Len()+len(b) > 2<<20 {
		w.exceeded = true
		w.cancel()
		return 0, errors.New("diagnostic response limit")
	}
	return w.body.Write(b)
}
func (w *boundedDiagnosticWriter) Flush() {}

type codexDiagnosticItem struct {
	Model          string  `json:"model"`
	Status         string  `json:"status"`
	Reason         string  `json:"reason,omitempty"`
	PredictedModel string  `json:"predicted_model,omitempty"`
	Probability    float64 `json:"probability,omitempty"`
	ParsedCount    int     `json:"parsed_number_count,omitempty"`
	HTTPStatus     int     `json:"http_status,omitempty"`
}

func (h *AccountHandler) DiagnoseModels(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var input struct {
		APIKeyID int64    `json:"api_key_id"`
		Models   []string `json:"models"`
	}
	if c.ShouldBindJSON(&input) != nil || input.APIKeyID <= 0 || len(input.Models) == 0 || len(input.Models) > 6 {
		response.BadRequest(c, "Select an API key and 1–6 models")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Error(c, 403, "Administrator required")
		return
	}
	if h.codexDiagnosticRouter == nil || h.codexDiagnosticKeys == nil {
		response.Error(c, 503, "Diagnostics unavailable")
		return
	}
	key, err := h.codexDiagnosticKeys.GetByID(c.Request.Context(), input.APIKeyID)
	if err != nil || key == nil || key.UserID != subject.UserID || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() || key.Key == "" {
		response.Error(c, 403, "API key unavailable or not owned by this administrator")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), id)
	if err != nil || account == nil || !account.IsOpenAI() {
		response.BadRequest(c, "Select an OpenAI account")
		return
	}
	models, err := service.ModelTraceGPTModels()
	if err != nil {
		response.Error(c, 503, "Fingerprint bank unavailable")
		return
	}
	allowed := map[string]bool{}
	for _, m := range models {
		allowed[m] = true
	}
	seen := map[string]bool{}
	for _, m := range input.Models {
		if !allowed[m] || seen[m] {
			response.BadRequest(c, "Unsupported or duplicate model")
			return
		}
		seen[m] = true
	}
	if _, busy := activeCodexDiagnostics.LoadOrStore(subject.UserID, true); busy {
		response.Error(c, 429, "A diagnostic is already running")
		return
	}
	defer activeCodexDiagnostics.Delete(subject.UserID)
	items := make([]codexDiagnosticItem, 0, len(input.Models))
	ctx, stop := context.WithTimeout(c.Request.Context(), 6*time.Minute)
	defer stop()
	for _, model := range input.Models {
		if ctx.Err() != nil {
			break
		}
		item := codexDiagnosticItem{Model: model, Status: "failed"}
		challenge, err := service.NewModelTraceChallenge()
		if err != nil {
			item.Reason = "challenge_generation_failed"
			items = append(items, item)
			continue
		}
		body, err := service.BuildStandaloneDiagnosticRequest(account, model, challenge)
		if err != nil {
			item.Reason = "request_build_failed"
			items = append(items, item)
			continue
		}
		modelCtx, cancel := context.WithTimeout(service.WithCodexTicketDiagnostic(ctx, id), 120*time.Second)
		req, _ := http.NewRequestWithContext(modelCtx, http.MethodPost, "/v1/responses", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+key.Key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("User-Agent", "Meridian-Model-Diagnostic/1.0")
		req.RemoteAddr = net.JoinHostPort(c.ClientIP(), "0")
		req.Host = c.Request.Host
		writer := &boundedDiagnosticWriter{header: make(http.Header), cancel: cancel}
		h.codexDiagnosticRouter.ServeHTTP(writer, req)
		cancel()
		item.HTTPStatus = writer.status
		if writer.exceeded {
			item.Reason = "response_too_large"
		} else if writer.status < 200 || writer.status >= 300 {
			item.Reason = "gateway_request_failed"
		} else {
			output, complete := codexDiagnosticOutput(writer.body.Bytes())
			if !complete {
				item.Reason = "response_incomplete"
			} else {
				prediction, _, err := service.ModelTracePredictCommitted(output, challenge.ExpectedCount)
				item.ParsedCount = prediction.ParsedCount
				if err != nil {
					item.Status = "uncertain"
					item.Reason = "insufficient_numbers"
				} else {
					item.PredictedModel = prediction.Model
					item.Probability = prediction.Probability
					if prediction.Probability < 0.97 {
						item.Status = "uncertain"
						item.Reason = "low_confidence"
					} else if prediction.Model == model {
						item.Status = "normal"
					} else {
						item.Status = "degraded"
						item.Reason = "fingerprint_mismatch"
					}
				}
			}
		}
		items = append(items, item)
	}
	response.Success(c, gin.H{"items": items, "canceled": ctx.Err() != nil, "bank_commit": service.ModelTraceBankCommit()})
}
