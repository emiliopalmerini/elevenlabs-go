package elevenlabs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUserParsesAccountMetadata(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/v1/user" {
			t.Fatalf("path = %s, want /v1/user", r.URL.Path)
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"user_id": "user_123",
			"subscription": {
				"tier": "creator",
				"character_count": 1234,
				"character_limit": 100000,
				"can_extend_character_limit": true,
				"allowed_to_extend_character_limit": true,
				"next_character_count_reset_unix": 1700000000,
				"voice_limit": 10,
				"max_voice_add_edits": 20,
				"voice_add_edit_counter": 3,
				"professional_voice_limit": 2,
				"can_extend_voice_limit": false,
				"can_use_instant_voice_cloning": true,
				"can_use_professional_voice_cloning": true,
				"currency": "usd",
				"status": "active"
			},
			"is_new_user": false,
			"xi_api_key": "secret-key",
			"can_use_delayed_payment_methods": true,
			"is_onboarding_completed": true,
			"first_name": "Ada",
			"created_at": 1689761411,
			"seat_type": "workspace_admin",
			"is_api_key_hashed": true,
			"xi_api_key_preview": "sk_...123",
			"show_compliance_terms": false,
			"available_models": ["eleven_multilingual_v2", "scribe_v1"],
			"next_invoice": {
				"amount_due_cents": 9900,
				"next_payment_attempt_unix": 1701000000
			}
		}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
	)

	user, err := client.GetUser(ctx)
	if err != nil {
		t.Fatalf("GetUser returned error: %v", err)
	}
	if user.UserID != "user_123" {
		t.Fatalf("UserID = %q, want user_123", user.UserID)
	}
	if user.Subscription == nil {
		t.Fatal("Subscription = nil, want subscription")
	}
	if user.Subscription.Tier != "creator" {
		t.Fatalf("Subscription.Tier = %q, want creator", user.Subscription.Tier)
	}
	if user.Subscription.CharacterCount != 1234 {
		t.Fatalf("Subscription.CharacterCount = %d, want 1234", user.Subscription.CharacterCount)
	}
	if user.FirstName != "Ada" {
		t.Fatalf("FirstName = %q, want Ada", user.FirstName)
	}
	if user.XIAPIKey != "secret-key" {
		t.Fatalf("XIAPIKey = %q, want secret-key", user.XIAPIKey)
	}
	if len(user.AvailableModels) != 2 || user.AvailableModels[1] != "scribe_v1" {
		t.Fatalf("AvailableModels = %#v, want scribe_v1 as second model", user.AvailableModels)
	}
	if user.NextInvoice == nil || user.NextInvoice.AmountDueCents != 9900 {
		t.Fatalf("NextInvoice = %+v, want amount due", user.NextInvoice)
	}
}

func TestGetUserWithResponseReturnsRawMetadata(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "req_user")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user_id":"user_123"}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
	)

	resp, err := client.GetUserWithResponse(ctx)
	if err != nil {
		t.Fatalf("GetUserWithResponse returned error: %v", err)
	}
	if resp.Data.UserID != "user_123" {
		t.Fatalf("UserID = %q, want user_123", resp.Data.UserID)
	}
	if resp.RawResponse.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.RawResponse.StatusCode, http.StatusOK)
	}
	if resp.RawResponse.Header.Get("X-Request-ID") != "req_user" {
		t.Fatalf("X-Request-ID = %q, want req_user", resp.RawResponse.Header.Get("X-Request-ID"))
	}
	if resp.RawResponse.URL != server.URL+"/v1/user" {
		t.Fatalf("URL = %q, want %s/v1/user", resp.RawResponse.URL, server.URL)
	}
}
