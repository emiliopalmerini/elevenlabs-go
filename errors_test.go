package elevenlabs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIErrorParsesProviderFailures(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		headers  map[string]string
		wantText []string
		check    func(*testing.T, *APIError)
	}{
		{
			name:   "quota exceeded",
			status: http.StatusPaymentRequired,
			body:   `{"detail":{"type":"invalid_request","code":"quota_exceeded","message":"This request exceeds your quota.","status":"quota_exceeded","request_id":"req_quota"}}`,
			headers: map[string]string{
				"x-trace-id": "trace_123",
			},
			wantText: []string{
				"ElevenLabs quota exceeded",
				"This request exceeds your quota.",
				"request_id: req_quota",
				"trace_id: trace_123",
			},
			check: func(t *testing.T, err *APIError) {
				t.Helper()
				if err.ProviderCode != "quota_exceeded" || err.RequestID != "req_quota" {
					t.Fatalf("api error = %+v", err)
				}
			},
		},
		{
			name:     "auth failure",
			status:   http.StatusUnauthorized,
			body:     `{"detail":{"message":"Invalid API key"}}`,
			wantText: []string{"ElevenLabs authentication failed", "Invalid API key"},
		},
		{
			name:     "forbidden",
			status:   http.StatusForbidden,
			body:     `{"detail":"IP address is not allowed"}`,
			wantText: []string{"ElevenLabs request forbidden", "IP address is not allowed"},
		},
		{
			name:     "not found",
			status:   http.StatusNotFound,
			body:     `{"detail":{"message":"Transcript not found"}}`,
			wantText: []string{"ElevenLabs resource not found", "Transcript not found"},
		},
		{
			name:   "validation array",
			status: http.StatusUnprocessableEntity,
			body: `{"detail":[
				{"loc":["body","file"],"msg":"Field required","type":"missing"},
				{"loc":["body","model_id"],"msg":"Input should be 'scribe_v2' or 'scribe_v1'","type":"enum"}
			]}`,
			wantText: []string{
				"ElevenLabs request validation failed",
				"body.file: Field required",
				"body.model_id: Input should be",
			},
			check: func(t *testing.T, err *APIError) {
				t.Helper()
				if len(err.Validation) != 2 {
					t.Fatalf("validation = %#v", err.Validation)
				}
			},
		},
		{
			name:   "rate limited",
			status: http.StatusTooManyRequests,
			body:   `{"detail":{"message":"Too many requests"}}`,
			headers: map[string]string{
				"retry-after": "30",
			},
			wantText: []string{
				"ElevenLabs rate limit exceeded",
				"Too many requests",
				"retry_after: 30",
			},
		},
		{
			name:     "invalid json fallback",
			status:   http.StatusInternalServerError,
			body:     "temporary upstream failure",
			wantText: []string{"ElevenLabs API returned 500 Internal Server Error", "temporary upstream failure"},
		},
		{
			name:     "oversized body fallback is compacted",
			status:   http.StatusBadGateway,
			body:     strings.Repeat("x", 5000) + "tail-marker",
			wantText: []string{"ElevenLabs API returned 502 Bad Gateway", strings.Repeat("x", 100)},
		},
		{
			name:     "redacts token-like values",
			status:   http.StatusBadRequest,
			body:     `{"detail":{"message":"authorization: secret-token"}}`,
			wantText: []string{"authorization: [REDACTED]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for name, value := range tt.headers {
					w.Header().Set(name, value)
				}
				http.Error(w, tt.body, tt.status)
			}))
			defer server.Close()

			client := newTestClient(t, server)
			_, err := client.Models.List(context.Background())
			if err == nil {
				t.Fatal("Models.List() error = nil, want API error")
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error %T does not expose *APIError", err)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error = %q, want substring %q", err.Error(), want)
				}
			}
			if strings.Contains(err.Error(), "tail-marker") {
				t.Fatalf("error = %q, want compacted body without tail marker", err.Error())
			}
			if tt.check != nil {
				tt.check(t, apiErr)
			}
		})
	}
}
