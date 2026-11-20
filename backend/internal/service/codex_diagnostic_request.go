package service

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"
	"time"
)

// Standalone read-only template; it has no ticket harvest, injection or refresh.
//
//go:embed codex_diagnostic_template.jsonl
var diagnosticTemplate []byte

func BuildStandaloneDiagnosticRequest(account *Account, model string, challenge ModelTraceChallenge) ([]byte, error) {
	location, err := time.LoadLocation(account.OpenAIRequestTimezone())
	if err != nil {
		return nil, err
	}
	render := strings.NewReplacer("{{CURRENT_DATE}}", time.Now().In(location).Format(time.DateOnly), "{{TIMEZONE}}", location.String(), "{{MODEL}}", model, "{{MODELTRACE_PROMPT}}", challenge.Prompt)
	input := []any{}
	instructions := ""
	scanner := bufio.NewScanner(bytes.NewReader(diagnosticTemplate))
	scanner.Buffer(make([]byte, 4096), 256<<10)
	for scanner.Scan() {
		var record struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, err
		}
		switch record.Type {
		case "session_meta":
			var p struct {
				Base struct {
					Text string `json:"text"`
				} `json:"base_instructions"`
			}
			if err := json.Unmarshal(record.Payload, &p); err != nil {
				return nil, err
			}
			instructions = render.Replace(p.Base.Text)
		case "response_item":
			var p struct {
				Type    string `json:"type"`
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			}
			if err := json.Unmarshal(record.Payload, &p); err != nil {
				return nil, err
			}
			parts := []any{}
			for _, part := range p.Content {
				parts = append(parts, map[string]any{"type": "input_text", "text": render.Replace(part.Text)})
			}
			input = append(input, map[string]any{"type": "message", "role": p.Role, "content": parts})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"model": model, "instructions": instructions, "input": input, "stream": true, "store": false})
}
