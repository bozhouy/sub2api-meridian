package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type localeAuditTransport struct{ receiver *httptest.Server }

func (t localeAuditTransport) Do(r *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	dst, err := url.Parse(t.receiver.URL)
	if err != nil {
		return nil, err
	}
	r.URL.Host, r.URL.Scheme = dst.Host, dst.Scheme
	return t.receiver.Client().Do(r)
}

func (t localeAuditTransport) DoWithTLS(r *http.Request, p string, id int64, n int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return t.Do(r, p, id, n)
}

func TestOpenAIRequestLocaleWireRoundTrip(t *testing.T) {
	const wantZone = "America/New_York"
	const wantLanguage = "en-US,en;q=0.9"
	for _, pass := range []bool{false, true} {
		t.Run(fmt.Sprintf("passthrough=%t", pass), func(t *testing.T) {
			type observation struct{ language, countryHeader, timezoneHeader, body string }
			received := make(chan observation, 1)
			up := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					http.Error(w, "bad test request", 400)
					return
				}
				received <- observation{req.Header.Get("Accept-Language"), req.Header.Get("CF-IPCountry"), req.Header.Get("X-Timezone"), string(body)}
				w.Header().Set("Content-Language", "fr-CA")
				w.Header().Set("X-Request-Id", "test-request-42")
				w.Header().Set("CF-IPCountry", "GB")
				w.Header().Set("X-Timezone", "Europe/London")
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_locale\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n")
			}))
			defer up.Close()
			body := []byte(`{"model":"gpt-5.4","stream":false,"instructions":"locale test","input":[{"role":"user","type":"message","content":[{"type":"input_text","text":"<environment_context><current_date>2001-01-01</current_date><timezone>Asia/Shanghai</timezone></environment_context>"},{"type":"input_text","text":"Example: Asia/Shanghai"}]}],"tools":[{"type":"web_search","user_location":{"type":"approximate","country":"CN","region":"Shanghai","city":"Shanghai","timezone":"Asia/Shanghai"}}]}`)
			out := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(out)
			c.Request = httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("Accept-Language", "zh-CN")
			c.Request.Header.Set("CF-IPCountry", "CN")
			c.Request.Header.Set("X-Timezone", "Asia/Shanghai")
			a := &Account{ID: 100, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"access_token": "fake", "chatgpt_account_id": "fake"}, Extra: map[string]any{"openai_passthrough": pass, "openai_request_timezone_mode": "fixed", "openai_request_timezone": "America/New_York"}}
			svc := &OpenAIGatewayService{httpUpstream: localeAuditTransport{up}, responseHeaderFilter: compileResponseHeaderFilter(&config.Config{})}
			_, err := svc.Forward(context.Background(), c, a, body)
			require.NoError(t, err)
			seen := <-received
			location := gjson.Get(seen.body, "tools.0.user_location")
			response := out.Result()
			defer response.Body.Close()
			t.Logf("sent language=%q location=%s env=%q; received HTTP %d Content-Language=%q X-Request-Id=%q", seen.language, location.Raw, gjson.Get(seen.body, "input.0.content.0.text"), response.StatusCode, response.Header.Get("Content-Language"), response.Header.Get("X-Request-Id"))
			require.Equal(t, wantLanguage, seen.language)
			require.Empty(t, seen.countryHeader)
			require.Empty(t, seen.timezoneHeader)
			require.Equal(t, "<environment_context><current_date>2001-01-01</current_date><timezone>"+wantZone+"</timezone></environment_context>", gjson.Get(seen.body, "input.0.content.0.text").String())
			require.Equal(t, "Example: Asia/Shanghai", gjson.Get(seen.body, "input.0.content.1.text").String())
			require.Equal(t, wantZone, location.Get("timezone").String())
			require.Equal(t, "CN", location.Get("country").String())
			require.Equal(t, "Shanghai", location.Get("region").String())
			require.Equal(t, "Shanghai", location.Get("city").String())
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.Equal(t, "fr-CA", response.Header.Get("Content-Language"))
			require.Equal(t, "test-request-42", response.Header.Get("X-Request-Id"))
			require.Empty(t, response.Header.Get("CF-IPCountry"))
			require.Empty(t, response.Header.Get("X-Timezone"))
		})
	}
}
