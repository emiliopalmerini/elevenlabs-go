package elevenlabs

import (
	"context"
)

// User contains account metadata returned by the ElevenLabs user endpoint.
type User struct {
	UserID                      string            `json:"user_id,omitempty"`
	Subscription                *UserSubscription `json:"subscription,omitempty"`
	IsNewUser                   bool              `json:"is_new_user,omitempty"`
	XIAPIKey                    string            `json:"xi_api_key,omitempty"`
	CanUseDelayedPaymentMethods bool              `json:"can_use_delayed_payment_methods,omitempty"`
	IsOnboardingCompleted       bool              `json:"is_onboarding_completed,omitempty"`
	FirstName                   string            `json:"first_name,omitempty"`
	CreatedAt                   int64             `json:"created_at,omitempty"`
	SeatType                    string            `json:"seat_type,omitempty"`
	IsAPIKeyHashed              bool              `json:"is_api_key_hashed,omitempty"`
	XIAPIKeyPreview             string            `json:"xi_api_key_preview,omitempty"`
	ShowComplianceTerms         bool              `json:"show_compliance_terms,omitempty"`
	AvailableModels             []string          `json:"available_models,omitempty"`
	NextInvoice                 *UserInvoice      `json:"next_invoice,omitempty"`
}

// UserSubscription contains subscription and quota metadata for a user.
type UserSubscription struct {
	Tier                                string       `json:"tier,omitempty"`
	CharacterCount                      int64        `json:"character_count,omitempty"`
	CharacterLimit                      int64        `json:"character_limit,omitempty"`
	CanExtendCharacterLimit             bool         `json:"can_extend_character_limit,omitempty"`
	AllowedToExtendCharacterLimit       bool         `json:"allowed_to_extend_character_limit,omitempty"`
	NextCharacterCountResetUnix         int64        `json:"next_character_count_reset_unix,omitempty"`
	VoiceLimit                          int64        `json:"voice_limit,omitempty"`
	MaxVoiceAddEdits                    int64        `json:"max_voice_add_edits,omitempty"`
	VoiceAddEditCounter                 int64        `json:"voice_add_edit_counter,omitempty"`
	ProfessionalVoiceLimit              int64        `json:"professional_voice_limit,omitempty"`
	CanExtendVoiceLimit                 bool         `json:"can_extend_voice_limit,omitempty"`
	CanUseInstantVoiceCloning           bool         `json:"can_use_instant_voice_cloning,omitempty"`
	CanUseProfessionalVoiceCloning      bool         `json:"can_use_professional_voice_cloning,omitempty"`
	Currency                            string       `json:"currency,omitempty"`
	Status                              string       `json:"status,omitempty"`
	BillingPeriod                       string       `json:"billing_period,omitempty"`
	CharacterRefreshPeriod              string       `json:"character_refresh_period,omitempty"`
	NextInvoice                         *UserInvoice `json:"next_invoice,omitempty"`
	HasOpenInvoices                     bool         `json:"has_open_invoices,omitempty"`
	CanUsePVCInstantly                  bool         `json:"can_use_pvc_instantly,omitempty"`
	CanUseVoiceDesign                   bool         `json:"can_use_voice_design,omitempty"`
	CanUseInstantVoiceCloningTrial      bool         `json:"can_use_instant_voice_cloning_trial,omitempty"`
	CanUseProfessionalVoiceCloningTrial bool         `json:"can_use_professional_voice_cloning_trial,omitempty"`
}

// UserInvoice contains upcoming invoice metadata.
type UserInvoice struct {
	AmountDueCents         int64 `json:"amount_due_cents,omitempty"`
	NextPaymentAttemptUnix int64 `json:"next_payment_attempt_unix,omitempty"`
}

// GetUser gets information about the authenticated user.
func (c *Client) GetUser(ctx context.Context) (*User, error) {
	resp, err := c.GetUserWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetUserWithResponse gets information about the authenticated user and returns
// HTTP response metadata.
func (c *Client) GetUserWithResponse(ctx context.Context) (*Response[*User], error) {
	var out User
	raw, err := c.getJSON(ctx, "/v1/user", &out)
	if err != nil {
		return nil, err
	}

	return &Response[*User]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}
