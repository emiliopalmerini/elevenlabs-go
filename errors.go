package elevenlabs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// APIError is returned when the ElevenLabs API responds with a non-2xx status.
type APIError struct {
	StatusCode      int
	Status          string
	Message         string
	Body            []byte
	ProviderType    string
	ProviderCode    string
	ProviderStatus  string
	ProviderMessage string
	RequestID       string
	TraceID         string
	RetryAfter      string
	Validation      []ValidationError
	RawResponse     RawResponse
}

// ValidationError contains validation details returned by the ElevenLabs API.
type ValidationError struct {
	Loc  []any  `json:"loc"`
	Msg  string `json:"msg"`
	Type string `json:"type"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("elevenlabs: api error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("elevenlabs: api error %d", e.StatusCode)
}

func newAPIError(statusCode int, status string, body []byte, raw RawResponse) *APIError {
	apiErr := &APIError{
		StatusCode:  statusCode,
		Status:      status,
		Body:        body,
		RequestID:   firstNonEmptyHeader(raw.Header, "request-id", "x-request-id"),
		TraceID:     raw.Header.Get("x-trace-id"),
		RetryAfter:  raw.Header.Get("retry-after"),
		RawResponse: raw,
	}
	parseAPIErrorBody(apiErr, body)
	apiErr.Message = firstNonEmptyString(
		apiErr.ProviderMessage,
		validationMessage(apiErr.Validation),
		compactErrorBody(body),
	)
	return apiErr
}

func parseAPIErrorBody(apiErr *APIError, body []byte) {
	if len(bytes.TrimSpace(body)) == 0 {
		return
	}
	var payload providerErrorBody
	if err := json.Unmarshal(body, &payload); err != nil {
		return
	}

	apiErr.ProviderType = payload.Type
	apiErr.ProviderCode = payload.Code
	apiErr.ProviderStatus = payload.Status
	apiErr.ProviderMessage = firstNonEmptyString(payload.Message, payload.Error)
	apiErr.RequestID = firstNonEmptyString(apiErr.RequestID, payload.RequestID)

	if len(payload.Detail) == 0 || string(payload.Detail) == "null" {
		return
	}

	var detail providerErrorDetail
	if err := json.Unmarshal(payload.Detail, &detail); err == nil {
		apiErr.ProviderType = firstNonEmptyString(apiErr.ProviderType, detail.Type)
		apiErr.ProviderCode = firstNonEmptyString(apiErr.ProviderCode, detail.Code)
		apiErr.ProviderStatus = firstNonEmptyString(apiErr.ProviderStatus, detail.Status)
		apiErr.ProviderMessage = firstNonEmptyString(apiErr.ProviderMessage, detail.Message, detail.Error)
		apiErr.RequestID = firstNonEmptyString(apiErr.RequestID, detail.RequestID)
		if detail.hasValue() {
			return
		}
	}

	var detailText string
	if err := json.Unmarshal(payload.Detail, &detailText); err == nil {
		apiErr.ProviderMessage = firstNonEmptyString(apiErr.ProviderMessage, detailText)
		return
	}

	var validation []ValidationError
	if err := json.Unmarshal(payload.Detail, &validation); err == nil {
		apiErr.Validation = validation
		return
	}
}

type providerErrorBody struct {
	Detail    json.RawMessage `json:"detail"`
	Type      string          `json:"type"`
	Code      string          `json:"code"`
	Status    string          `json:"status"`
	Message   string          `json:"message"`
	Error     string          `json:"error"`
	RequestID string          `json:"request_id"`
}

type providerErrorDetail struct {
	Type      string `json:"type"`
	Code      string `json:"code"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Error     string `json:"error"`
	RequestID string `json:"request_id"`
}

func (d providerErrorDetail) hasValue() bool {
	return d.Type != "" ||
		d.Code != "" ||
		d.Status != "" ||
		d.Message != "" ||
		d.Error != "" ||
		d.RequestID != ""
}

func firstNonEmptyHeader(headers http.Header, names ...string) string {
	for _, name := range names {
		if value := headers.Get(name); strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func validationMessage(items []ValidationError) string {
	summaries := validationSummaries(items, 3)
	return strings.Join(summaries, "; ")
}

func validationSummaries(items []ValidationError, maxItems int) []string {
	if maxItems <= 0 {
		return nil
	}

	summaries := make([]string, 0, len(items))
	for _, item := range items {
		msg := firstNonEmptyString(item.Msg, item.Type)
		if msg == "" {
			continue
		}
		loc := validationLoc(item.Loc)
		if loc != "" {
			msg = loc + ": " + msg
		}
		summaries = append(summaries, msg)
		if len(summaries) == maxItems {
			break
		}
	}
	if len(items) > maxItems {
		summaries = append(summaries, fmt.Sprintf("%d more validation errors", len(items)-maxItems))
	}
	return summaries
}

func validationLoc(parts []any) string {
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		switch value := part.(type) {
		case string:
			if value != "" {
				labels = append(labels, value)
			}
		case float64:
			labels = append(labels, fmt.Sprintf("%.0f", value))
		default:
			labels = append(labels, fmt.Sprint(value))
		}
	}
	return strings.Join(labels, ".")
}

func compactErrorBody(body []byte) string {
	const maxErrorBodyBytes = 4096
	body = bytes.TrimSpace(body)
	if len(body) > maxErrorBodyBytes {
		body = body[:maxErrorBodyBytes]
	}
	return string(body)
}
