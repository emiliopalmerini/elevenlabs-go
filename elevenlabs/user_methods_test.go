package elevenlabs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetParsesAccountMetadata(t *testing.T) {
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
				"tier": "trial",
				"character_count": 17231,
				"character_limit": 100000,
				"max_character_limit_extension": 10000,
				"max_credit_limit_extension": 10000,
				"can_extend_character_limit": true,
				"allowed_to_extend_character_limit": true,
				"next_character_count_reset_unix": 1738356858,
				"voice_slots_used": 1,
				"professional_voice_slots_used": 0,
				"voice_limit": 10,
				"max_voice_add_edits": 20,
				"voice_add_edit_counter": 3,
				"professional_voice_limit": 2,
				"can_extend_voice_limit": false,
				"can_use_instant_voice_cloning": true,
				"can_use_professional_voice_cloning": true,
				"currency": "usd",
				"current_overage": {
					"amount": "0",
					"currency": "usd"
				},
				"status": "active",
				"billing_period": "monthly_period",
				"character_refresh_period": "monthly_period"
			},
			"is_new_user": false,
			"xi_api_key": "secret-key",
			"can_use_delayed_payment_methods": true,
			"is_onboarding_completed": true,
			"is_onboarding_checklist_completed": true,
			"show_compliance_terms": false,
			"first_name": "Ada",
			"is_api_key_hashed": true,
			"xi_api_key_preview": "sk_...123",
			"referral_link_code": "ref_123",
			"partnerstack_partner_default_link": "https://example.com/partner",
			"created_at": 1689761411,
			"seat_type": "workspace_admin"
		}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
	)

	got, err := client.User.Get(ctx)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.UserID != "user_123" {
		t.Fatalf("UserID = %q, want user_123", got.UserID)
	}
	if got.FirstName == nil || *got.FirstName != "Ada" {
		t.Fatalf("FirstName = %v, want Ada", got.FirstName)
	}
	if got.Subscription == nil {
		t.Fatal("Subscription = nil, want subscription")
	}
	if got.Subscription.Tier != "trial" {
		t.Fatalf("Subscription.Tier = %q, want trial", got.Subscription.Tier)
	}
	if got.Subscription.MaxCreditLimitExtension.Value == nil || *got.Subscription.MaxCreditLimitExtension.Value != 10000 {
		t.Fatalf("MaxCreditLimitExtension = %+v, want value 10000", got.Subscription.MaxCreditLimitExtension)
	}
	if got.Subscription.CurrentOverage == nil || got.Subscription.CurrentOverage.Currency != "usd" {
		t.Fatalf("CurrentOverage = %+v, want usd overage", got.Subscription.CurrentOverage)
	}
}

func TestGetWithResponseReturnsRawMetadata(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "req_user")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"user_id": "user_123",
			"subscription": {
				"tier": "free",
				"character_count": 0,
				"character_limit": 10000,
				"max_character_limit_extension": 0,
				"max_credit_limit_extension": 0,
				"can_extend_character_limit": false,
				"allowed_to_extend_character_limit": false,
				"voice_slots_used": 0,
				"professional_voice_slots_used": 0,
				"voice_limit": 3,
				"voice_add_edit_counter": 0,
				"professional_voice_limit": 0,
				"can_extend_voice_limit": false,
				"can_use_instant_voice_cloning": false,
				"can_use_professional_voice_cloning": false,
				"current_overage": {
					"amount": "0",
					"currency": "usd"
				},
				"status": "free"
			},
			"is_new_user": false,
			"can_use_delayed_payment_methods": false,
			"is_onboarding_completed": true,
			"is_onboarding_checklist_completed": true,
			"created_at": 1689761411,
			"seat_type": "workspace_admin"
		}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
	)

	resp, err := client.User.GetWithResponse(ctx)
	if err != nil {
		t.Fatalf("GetWithResponse returned error: %v", err)
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

func TestGetSubscriptionParsesExtendedMetadata(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/v1/user/subscription" {
			t.Fatalf("path = %s, want /v1/user/subscription", r.URL.Path)
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tier": "starter",
			"character_count": 1000,
			"character_limit": 10000,
			"max_character_limit_extension": null,
			"max_credit_limit_extension": "unlimited",
			"can_extend_character_limit": true,
			"allowed_to_extend_character_limit": true,
			"next_character_count_reset_unix": 1738356858,
			"voice_slots_used": 1,
			"professional_voice_slots_used": 0,
			"voice_limit": 10,
			"max_voice_add_edits": null,
			"voice_add_edit_counter": 0,
			"professional_voice_limit": 2,
			"can_extend_voice_limit": false,
			"can_use_instant_voice_cloning": true,
			"can_use_professional_voice_cloning": false,
			"currency": "usd",
			"current_overage": {
				"amount": "12.34",
				"currency": "usd"
			},
			"status": "active",
			"billing_period": "monthly_period",
			"character_refresh_period": "monthly_period",
			"next_invoice": {
				"amount_due_cents": 1000,
				"subtotal_cents": 1200,
				"tax_cents": 100,
				"discount_percent_off": null,
				"discount_amount_off": null,
				"discounts": [
					{
						"discount_percent_off": 20.0,
						"discount_amount_off": null
					}
				],
				"next_payment_attempt_unix": 1739000000,
				"payment_intent_status": "processing",
				"payment_intent_statusses": ["processing", "succeeded"]
			},
			"open_invoices": [
				{
					"amount_due_cents": 500,
					"discounts": [],
					"next_payment_attempt_unix": -1,
					"payment_intent_status": null,
					"payment_intent_statusses": []
				}
			],
			"has_open_invoices": true,
			"pending_change": {
				"kind": "change",
				"next_tier": "creator",
				"next_billing_period": "annual_period",
				"timestamp_seconds": 1739100000
			},
			"has_used_starter_coupon_on_account": true,
			"has_used_creator_coupon_on_account": false
		}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
	)

	got, err := client.User.GetSubscription(ctx)
	if err != nil {
		t.Fatalf("GetSubscription returned error: %v", err)
	}
	if got.Tier != "starter" {
		t.Fatalf("Tier = %q, want starter", got.Tier)
	}
	if !got.MaxCreditLimitExtension.Unlimited {
		t.Fatalf("MaxCreditLimitExtension = %+v, want unlimited", got.MaxCreditLimitExtension)
	}
	if got.NextInvoice == nil || got.NextInvoice.AmountDueCents != 1000 {
		t.Fatalf("NextInvoice = %+v, want amount due", got.NextInvoice)
	}
	if len(got.NextInvoice.Discounts) != 1 || got.NextInvoice.Discounts[0].DiscountPercentOff == nil || *got.NextInvoice.Discounts[0].DiscountPercentOff != 20 {
		t.Fatalf("Discounts = %#v, want 20 percent discount", got.NextInvoice.Discounts)
	}
	if len(got.OpenInvoices) != 1 || got.OpenInvoices[0].AmountDueCents != 500 {
		t.Fatalf("OpenInvoices = %#v, want one open invoice", got.OpenInvoices)
	}
	if got.PendingChange == nil || got.PendingChange.NextTier != "creator" {
		t.Fatalf("PendingChange = %+v, want creator change", got.PendingChange)
	}
}

func TestGetSubscriptionWithResponseReturnsRawMetadata(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "req_subscription")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tier": "free",
			"character_count": 0,
			"character_limit": 10000,
			"max_character_limit_extension": 0,
			"max_credit_limit_extension": 0,
			"can_extend_character_limit": false,
			"allowed_to_extend_character_limit": false,
			"voice_slots_used": 0,
			"professional_voice_slots_used": 0,
			"voice_limit": 3,
			"voice_add_edit_counter": 0,
			"professional_voice_limit": 0,
			"can_extend_voice_limit": false,
			"can_use_instant_voice_cloning": false,
			"can_use_professional_voice_cloning": false,
			"current_overage": {
				"amount": "0",
				"currency": "usd"
			},
			"status": "free",
			"open_invoices": [],
			"has_open_invoices": false
		}`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
	)

	resp, err := client.User.GetSubscriptionWithResponse(ctx)
	if err != nil {
		t.Fatalf("GetSubscriptionWithResponse returned error: %v", err)
	}
	if resp.Data.Tier != "free" {
		t.Fatalf("Tier = %q, want free", resp.Data.Tier)
	}
	if resp.RawResponse.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.RawResponse.StatusCode, http.StatusOK)
	}
	if resp.RawResponse.Header.Get("X-Request-ID") != "req_subscription" {
		t.Fatalf("X-Request-ID = %q, want req_subscription", resp.RawResponse.Header.Get("X-Request-ID"))
	}
	if resp.RawResponse.URL != server.URL+"/v1/user/subscription" {
		t.Fatalf("URL = %q, want %s/v1/user/subscription", resp.RawResponse.URL, server.URL)
	}
}

func TestCreditLimitExtensionRoundTripsDocumentedUnion(t *testing.T) {
	var numeric CreditLimitExtension
	if err := json.Unmarshal([]byte(`10000`), &numeric); err != nil {
		t.Fatalf("decode numeric extension: %v", err)
	}
	if numeric.Value == nil || *numeric.Value != 10000 {
		t.Fatalf("numeric extension = %+v, want value 10000", numeric)
	}
	data, err := json.Marshal(numeric)
	if err != nil {
		t.Fatalf("encode numeric extension: %v", err)
	}
	if string(data) != "10000" {
		t.Fatalf("encoded numeric extension = %s, want 10000", data)
	}

	var unlimited CreditLimitExtension
	if err := json.Unmarshal([]byte(`"unlimited"`), &unlimited); err != nil {
		t.Fatalf("decode unlimited extension: %v", err)
	}
	if !unlimited.Unlimited {
		t.Fatalf("unlimited extension = %+v, want Unlimited", unlimited)
	}
	data, err = json.Marshal(unlimited)
	if err != nil {
		t.Fatalf("encode unlimited extension: %v", err)
	}
	if string(data) != `"unlimited"` {
		t.Fatalf("encoded unlimited extension = %s, want \"unlimited\"", data)
	}
}
