package elevenlabs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var tokenLike = regexp.MustCompile(`(?i)(xi-api-key|api[_-]?key|authorization|token)(["':=\s]+)([^"',\s]+)`)

// RequestError wraps transport-level failures before an HTTP response is
// available.
type RequestError struct {
	Method string
	URL    string
	Err    error
}

func (e *RequestError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("elevenlabs: %s %s failed: %v", e.Method, e.URL, e.Err)
}

func (e *RequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// APIError is returned for non-2xx HTTP responses from ElevenLabs.
type APIError struct {
	StatusCode      int
	Status          string
	ProviderType    string
	ProviderCode    string
	ProviderStatus  string
	ProviderMessage string
	ProviderParam   string
	RequestID       string
	TraceID         string
	RetryAfter      string
	Body            string
	Validation      []string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return e.message()
}

func newAPIError(res *http.Response, body []byte) *APIError {
	apiErr := parseAPIError(res, body)
	apiErr.redact()
	return apiErr
}

func parseAPIError(res *http.Response, body []byte) *APIError {
	apiErr := &APIError{
		StatusCode: res.StatusCode,
		Status:     res.Status,
		RequestID:  firstNonEmpty(res.Header.Get("request-id"), res.Header.Get("x-request-id")),
		TraceID:    res.Header.Get("x-trace-id"),
		RetryAfter: res.Header.Get("retry-after"),
		Body:       redact(compactBody(body)),
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return apiErr
	}

	var parsed providerErrorBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return apiErr
	}
	applyProviderError(apiErr, parsed)
	if len(parsed.Detail) == 0 {
		return apiErr
	}

	var detail providerErrorDetail
	if err := json.Unmarshal(parsed.Detail, &detail); err == nil {
		applyProviderDetail(apiErr, detail)
		return apiErr
	}
	var detailText string
	if err := json.Unmarshal(parsed.Detail, &detailText); err == nil {
		apiErr.ProviderMessage = firstNonEmpty(apiErr.ProviderMessage, detailText)
		return apiErr
	}
	var validation []validationError
	if err := json.Unmarshal(parsed.Detail, &validation); err == nil {
		apiErr.Validation = summarizeValidation(validation)
		if len(apiErr.Validation) > 0 {
			apiErr.ProviderMessage = strings.Join(apiErr.Validation, "; ")
		}
	}
	return apiErr
}

type providerErrorBody struct {
	Detail    json.RawMessage `json:"detail"`
	Type      string          `json:"type"`
	Code      string          `json:"code"`
	Status    string          `json:"status"`
	Message   string          `json:"message"`
	Error     string          `json:"error"`
	Param     string          `json:"param"`
	RequestID string          `json:"request_id"`
}

type providerErrorDetail struct {
	Type      string `json:"type"`
	Code      string `json:"code"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Error     string `json:"error"`
	Param     string `json:"param"`
	RequestID string `json:"request_id"`
}

type validationError struct {
	Loc  []any  `json:"loc"`
	Msg  string `json:"msg"`
	Type string `json:"type"`
}

func applyProviderError(apiErr *APIError, parsed providerErrorBody) {
	apiErr.ProviderType = firstNonEmpty(apiErr.ProviderType, parsed.Type)
	apiErr.ProviderCode = firstNonEmpty(apiErr.ProviderCode, parsed.Code)
	apiErr.ProviderStatus = firstNonEmpty(apiErr.ProviderStatus, parsed.Status)
	apiErr.ProviderMessage = firstNonEmpty(apiErr.ProviderMessage, parsed.Message, parsed.Error)
	apiErr.ProviderParam = firstNonEmpty(apiErr.ProviderParam, parsed.Param)
	apiErr.RequestID = firstNonEmpty(apiErr.RequestID, parsed.RequestID)
}

func applyProviderDetail(apiErr *APIError, detail providerErrorDetail) {
	apiErr.ProviderType = firstNonEmpty(apiErr.ProviderType, detail.Type)
	apiErr.ProviderCode = firstNonEmpty(apiErr.ProviderCode, detail.Code)
	apiErr.ProviderStatus = firstNonEmpty(apiErr.ProviderStatus, detail.Status)
	apiErr.ProviderMessage = firstNonEmpty(apiErr.ProviderMessage, detail.Message, detail.Error)
	apiErr.ProviderParam = firstNonEmpty(apiErr.ProviderParam, detail.Param)
	apiErr.RequestID = firstNonEmpty(apiErr.RequestID, detail.RequestID)
}

func (e *APIError) redact() {
	e.ProviderMessage = redact(e.ProviderMessage)
	e.ProviderParam = redact(e.ProviderParam)
	for i, item := range e.Validation {
		e.Validation[i] = redact(item)
	}
}

func (e *APIError) message() string {
	message := firstNonEmpty(e.ProviderMessage, e.Body)
	switch {
	case e.ProviderCode == "quota_exceeded" || e.StatusCode == http.StatusPaymentRequired:
		return withRequestMeta("ElevenLabs quota exceeded", message, e)
	case e.StatusCode == http.StatusUnauthorized:
		return withRequestMeta("ElevenLabs authentication failed", message, e)
	case e.StatusCode == http.StatusForbidden:
		return withRequestMeta("ElevenLabs request forbidden", message, e)
	case e.StatusCode == http.StatusNotFound:
		return withRequestMeta("ElevenLabs resource not found", message, e)
	case e.StatusCode == http.StatusTooManyRequests:
		return withRequestMeta("ElevenLabs rate limit exceeded", message, e)
	case e.StatusCode == http.StatusBadRequest || e.StatusCode == http.StatusUnprocessableEntity:
		if len(e.Validation) > 0 {
			return withRequestMeta("ElevenLabs request validation failed", strings.Join(e.Validation, "; "), e)
		}
		return withRequestMeta(fmt.Sprintf("ElevenLabs API returned %s", e.Status), message, e)
	default:
		return withRequestMeta(fmt.Sprintf("ElevenLabs API returned %s", e.Status), message, e)
	}
}

func withRequestMeta(prefix, message string, apiErr *APIError) string {
	var b strings.Builder
	b.WriteString(prefix)
	if strings.TrimSpace(message) != "" {
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(message))
	}
	var meta []string
	if apiErr.RequestID != "" {
		meta = append(meta, "request_id: "+apiErr.RequestID)
	}
	if apiErr.TraceID != "" {
		meta = append(meta, "trace_id: "+apiErr.TraceID)
	}
	if apiErr.RetryAfter != "" {
		meta = append(meta, "retry_after: "+apiErr.RetryAfter)
	}
	if len(meta) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(meta, ", "))
		b.WriteString(")")
	}
	return b.String()
}

func summarizeValidation(items []validationError) []string {
	const maxValidationItems = 3
	summaries := make([]string, 0, len(items))
	for _, item := range items {
		msg := firstNonEmpty(item.Msg, item.Type)
		if msg == "" {
			continue
		}
		loc := formatValidationLoc(item.Loc)
		if loc != "" {
			msg = loc + ": " + msg
		}
		summaries = append(summaries, msg)
		if len(summaries) == maxValidationItems {
			break
		}
	}
	if len(items) > maxValidationItems {
		summaries = append(summaries, fmt.Sprintf("%d more validation errors", len(items)-maxValidationItems))
	}
	return summaries
}

func formatValidationLoc(parts []any) string {
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		switch v := part.(type) {
		case string:
			if v != "" {
				labels = append(labels, v)
			}
		case float64:
			labels = append(labels, fmt.Sprintf("%.0f", v))
		default:
			labels = append(labels, fmt.Sprint(v))
		}
	}
	return strings.Join(labels, ".")
}

func compactBody(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ""
	}
	if len(body) > 4096 {
		body = body[:4096]
	}
	return string(body)
}

func redact(s string) string {
	if s == "" {
		return s
	}
	redacted := tokenLike.ReplaceAllString(s, `$1$2[REDACTED]`)
	if len(redacted) > 4096 {
		return strings.TrimSpace(redacted[:4096]) + "..."
	}
	return redacted
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
