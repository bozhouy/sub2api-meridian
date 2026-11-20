package service

import (
	"net/http"
	"strings"
)

func normalizeOpenAIRequestLanguage(account *Account, headers http.Header) {
	if account == nil || !account.IsOpenAI() {
		return
	}
	if enabled, ok := account.Extra["openai_request_language_enabled"].(bool); ok && !enabled {
		return
	}
	present := false
	for key := range headers {
		if strings.EqualFold(key, "Accept-Language") {
			present = true
			delete(headers, key)
		}
	}
	if present {
		headers.Set("Accept-Language", "en-US,en;q=0.9")
	}
}
