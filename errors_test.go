package elevenlabs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIErrorParsesProviderMetadata(t *testing.T) {
	apiErr := apiErrorFromListModels(t, http.StatusUnauthorized, map[string]string{
		"request-id":  "req_header",
		"x-trace-id":  "trace_123",
		"retry-after": "30",
	}, `{"detail":{"type":"invalid_request","code":"quota_exceeded","message":"This request exceeds your quota","status":"quota_exceeded","request_id":"req_body"}}`)

	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
	if apiErr.ProviderType != "invalid_request" {
		t.Fatalf("ProviderType = %q, want invalid_request", apiErr.ProviderType)
	}
	if apiErr.ProviderCode != "quota_exceeded" {
		t.Fatalf("ProviderCode = %q, want quota_exceeded", apiErr.ProviderCode)
	}
	if apiErr.ProviderStatus != "quota_exceeded" {
		t.Fatalf("ProviderStatus = %q, want quota_exceeded", apiErr.ProviderStatus)
	}
	if apiErr.ProviderMessage != "This request exceeds your quota" {
		t.Fatalf("ProviderMessage = %q, want quota message", apiErr.ProviderMessage)
	}
	if apiErr.Message != "This request exceeds your quota" {
		t.Fatalf("Message = %q, want quota message", apiErr.Message)
	}
	if apiErr.RequestID != "req_header" {
		t.Fatalf("RequestID = %q, want req_header", apiErr.RequestID)
	}
	if apiErr.TraceID != "trace_123" {
		t.Fatalf("TraceID = %q, want trace_123", apiErr.TraceID)
	}
	if apiErr.RetryAfter != "30" {
		t.Fatalf("RetryAfter = %q, want 30", apiErr.RetryAfter)
	}
	if apiErr.RawResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("RawResponse.StatusCode = %d, want %d", apiErr.RawResponse.StatusCode, http.StatusUnauthorized)
	}
	if apiErr.RawResponse.Header.Get("request-id") != "req_header" {
		t.Fatalf("RawResponse request-id = %q, want req_header", apiErr.RawResponse.Header.Get("request-id"))
	}
}

func TestAPIErrorParsesTopLevelProviderFields(t *testing.T) {
	apiErr := apiErrorFromListModels(t, http.StatusForbidden, map[string]string{
		"x-request-id": "req_header_alt",
	}, `{"type":"invalid_request","code":"ip_not_allowed","status":"forbidden","message":"IP address is not allowed","request_id":"req_body"}`)

	if apiErr.ProviderType != "invalid_request" {
		t.Fatalf("ProviderType = %q, want invalid_request", apiErr.ProviderType)
	}
	if apiErr.ProviderCode != "ip_not_allowed" {
		t.Fatalf("ProviderCode = %q, want ip_not_allowed", apiErr.ProviderCode)
	}
	if apiErr.ProviderStatus != "forbidden" {
		t.Fatalf("ProviderStatus = %q, want forbidden", apiErr.ProviderStatus)
	}
	if apiErr.ProviderMessage != "IP address is not allowed" {
		t.Fatalf("ProviderMessage = %q, want IP address message", apiErr.ProviderMessage)
	}
	if apiErr.RequestID != "req_header_alt" {
		t.Fatalf("RequestID = %q, want req_header_alt", apiErr.RequestID)
	}
}

func TestAPIErrorParsesValidationDetail(t *testing.T) {
	apiErr := apiErrorFromListModels(t, http.StatusUnprocessableEntity, nil, `{"detail":[{"loc":["body","file"],"msg":"Field required","type":"missing"},{"loc":["body",2],"msg":"Bad item","type":"value_error"}]}`)

	if len(apiErr.Validation) != 2 {
		t.Fatalf("Validation length = %d, want 2", len(apiErr.Validation))
	}
	if apiErr.Validation[0].Msg != "Field required" {
		t.Fatalf("Validation[0].Msg = %q, want Field required", apiErr.Validation[0].Msg)
	}
	if len(apiErr.Validation[0].Loc) != 2 || apiErr.Validation[0].Loc[0] != "body" || apiErr.Validation[0].Loc[1] != "file" {
		t.Fatalf("Validation[0].Loc = %#v, want body.file", apiErr.Validation[0].Loc)
	}
	for _, want := range []string{"body.file: Field required", "body.2: Bad item"} {
		if !strings.Contains(apiErr.Message, want) {
			t.Fatalf("Message = %q, want substring %q", apiErr.Message, want)
		}
	}
}

func TestAPIErrorParsesStringDetail(t *testing.T) {
	apiErr := apiErrorFromListModels(t, http.StatusBadRequest, nil, `{"detail":"bad request"}`)

	if apiErr.ProviderMessage != "bad request" {
		t.Fatalf("ProviderMessage = %q, want bad request", apiErr.ProviderMessage)
	}
	if apiErr.Message != "bad request" {
		t.Fatalf("Message = %q, want bad request", apiErr.Message)
	}
}

func TestAPIErrorFallsBackToRawBody(t *testing.T) {
	apiErr := apiErrorFromListModels(t, http.StatusBadGateway, nil, "temporary upstream failure")

	if apiErr.Message != "temporary upstream failure" {
		t.Fatalf("Message = %q, want raw body fallback", apiErr.Message)
	}
	if string(apiErr.Body) != "temporary upstream failure" {
		t.Fatalf("Body = %q, want raw body", string(apiErr.Body))
	}
}

func TestAPIErrorUsesBodyRequestIDWhenHeaderMissing(t *testing.T) {
	apiErr := apiErrorFromListModels(t, http.StatusForbidden, nil, `{"message":"forbidden","request_id":"req_body"}`)

	if apiErr.RequestID != "req_body" {
		t.Fatalf("RequestID = %q, want req_body", apiErr.RequestID)
	}
}

func TestAPIErrorCompactsRawBodyFallbackMessage(t *testing.T) {
	body := strings.Repeat("x", 5000)
	apiErr := apiErrorFromListModels(t, http.StatusBadGateway, nil, body)

	if len(apiErr.Message) != 4096 {
		t.Fatalf("Message length = %d, want 4096", len(apiErr.Message))
	}
	if len(apiErr.Body) != 5000 {
		t.Fatalf("Body length = %d, want 5000", len(apiErr.Body))
	}
}

func apiErrorFromListModels(t *testing.T, statusCode int, headers map[string]string, body string) *APIError {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/v1/models" {
			t.Fatalf("request uri = %s, want /v1/models", r.URL.RequestURI())
		}
		for name, value := range headers {
			w.Header().Set(name, value)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithoutRetries())
	_, err := client.ListModels(context.Background())
	if err == nil {
		t.Fatal("ListModels error = nil, want API error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	return apiErr
}
