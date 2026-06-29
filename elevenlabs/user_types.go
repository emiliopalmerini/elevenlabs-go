package elevenlabs

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// User contains account metadata returned by the ElevenLabs user endpoint.
type User struct {
	UserID                         string        `json:"user_id,omitempty"`
	Subscription                   *Subscription `json:"subscription,omitempty"`
	IsNewUser                      bool          `json:"is_new_user,omitempty"`
	XIAPIKey                       *string       `json:"xi_api_key,omitempty"`
	CanUseDelayedPaymentMethods    bool          `json:"can_use_delayed_payment_methods,omitempty"`
	IsOnboardingCompleted          bool          `json:"is_onboarding_completed,omitempty"`
	IsOnboardingChecklistCompleted bool          `json:"is_onboarding_checklist_completed,omitempty"`
	ShowComplianceTerms            bool          `json:"show_compliance_terms,omitempty"`
	FirstName                      *string       `json:"first_name,omitempty"`
	IsAPIKeyHashed                 bool          `json:"is_api_key_hashed,omitempty"`
	XIAPIKeyPreview                *string       `json:"xi_api_key_preview,omitempty"`
	ReferralLinkCode               *string       `json:"referral_link_code,omitempty"`
	PartnerstackPartnerDefaultLink *string       `json:"partnerstack_partner_default_link,omitempty"`
	CreatedAt                      int64         `json:"created_at,omitempty"`
	SeatType                       string        `json:"seat_type,omitempty"`

	// AvailableModels and NextInvoice are retained for compatibility with
	// responses previously returned by the user endpoint.
	AvailableModels []string `json:"available_models,omitempty"`
	NextInvoice     *Invoice `json:"next_invoice,omitempty"`
}

// Subscription contains subscription and quota metadata for a user.
type Subscription struct {
	Tier                                string               `json:"tier,omitempty"`
	CharacterCount                      int64                `json:"character_count,omitempty"`
	CharacterLimit                      int64                `json:"character_limit,omitempty"`
	MaxCharacterLimitExtension          *int64               `json:"max_character_limit_extension,omitempty"`
	MaxCreditLimitExtension             CreditLimitExtension `json:"max_credit_limit_extension,omitempty"`
	CanExtendCharacterLimit             bool                 `json:"can_extend_character_limit,omitempty"`
	AllowedToExtendCharacterLimit       bool                 `json:"allowed_to_extend_character_limit,omitempty"`
	NextCharacterCountResetUnix         *int64               `json:"next_character_count_reset_unix,omitempty"`
	VoiceSlotsUsed                      int64                `json:"voice_slots_used,omitempty"`
	ProfessionalVoiceSlotsUsed          int64                `json:"professional_voice_slots_used,omitempty"`
	VoiceLimit                          int64                `json:"voice_limit,omitempty"`
	MaxVoiceAddEdits                    *int64               `json:"max_voice_add_edits,omitempty"`
	VoiceAddEditCounter                 int64                `json:"voice_add_edit_counter,omitempty"`
	ProfessionalVoiceLimit              int64                `json:"professional_voice_limit,omitempty"`
	CanExtendVoiceLimit                 bool                 `json:"can_extend_voice_limit,omitempty"`
	CanUseInstantVoiceCloning           bool                 `json:"can_use_instant_voice_cloning,omitempty"`
	CanUseProfessionalVoiceCloning      bool                 `json:"can_use_professional_voice_cloning,omitempty"`
	Currency                            *string              `json:"currency,omitempty"`
	CurrentOverage                      *Price               `json:"current_overage,omitempty"`
	Status                              string               `json:"status,omitempty"`
	BillingPeriod                       *string              `json:"billing_period,omitempty"`
	CharacterRefreshPeriod              *string              `json:"character_refresh_period,omitempty"`
	NextInvoice                         *Invoice             `json:"next_invoice,omitempty"`
	OpenInvoices                        []Invoice            `json:"open_invoices,omitempty"`
	HasOpenInvoices                     bool                 `json:"has_open_invoices,omitempty"`
	PendingChange                       *PendingChange       `json:"pending_change,omitempty"`
	HasUsedStarterCouponOnAccount       bool                 `json:"has_used_starter_coupon_on_account,omitempty"`
	HasUsedCreatorCouponOnAccount       bool                 `json:"has_used_creator_coupon_on_account,omitempty"`
	CanUsePVCInstantly                  bool                 `json:"can_use_pvc_instantly,omitempty"`
	CanUseVoiceDesign                   bool                 `json:"can_use_voice_design,omitempty"`
	CanUseInstantVoiceCloningTrial      bool                 `json:"can_use_instant_voice_cloning_trial,omitempty"`
	CanUseProfessionalVoiceCloningTrial bool                 `json:"can_use_professional_voice_cloning_trial,omitempty"`
}

// CreditLimitExtension is the user's overage cap. Unlimited is true when the
// API returns the string value "unlimited"; otherwise Value contains the cap.
type CreditLimitExtension struct {
	Value     *int64
	Unlimited bool
}

// UnmarshalJSON decodes the documented integer or "unlimited" string union.
func (e *CreditLimitExtension) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*e = CreditLimitExtension{}
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		if value != "unlimited" {
			return fmt.Errorf("elevenlabs: unsupported credit limit extension %q", value)
		}
		*e = CreditLimitExtension{Unlimited: true}
		return nil
	}

	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("elevenlabs: decode credit limit extension: %w", err)
	}
	*e = CreditLimitExtension{Value: &n}
	return nil
}

// MarshalJSON encodes the documented integer or "unlimited" string union.
func (e CreditLimitExtension) MarshalJSON() ([]byte, error) {
	if e.Unlimited {
		return json.Marshal("unlimited")
	}
	if e.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*e.Value)
}

// Price contains a currency/amount pair.
type Price struct {
	Amount   string `json:"amount,omitempty"`
	Currency string `json:"currency,omitempty"`
}

// Invoice contains invoice metadata returned with extended subscription data.
type Invoice struct {
	AmountDueCents         int64             `json:"amount_due_cents,omitempty"`
	SubtotalCents          *int64            `json:"subtotal_cents,omitempty"`
	TaxCents               *int64            `json:"tax_cents,omitempty"`
	DiscountPercentOff     *float64          `json:"discount_percent_off,omitempty"`
	DiscountAmountOff      *float64          `json:"discount_amount_off,omitempty"`
	Discounts              []InvoiceDiscount `json:"discounts,omitempty"`
	NextPaymentAttemptUnix int64             `json:"next_payment_attempt_unix,omitempty"`
	PaymentIntentStatus    *string           `json:"payment_intent_status,omitempty"`
	PaymentIntentStatuses  []string          `json:"payment_intent_statusses,omitempty"`
}

// InvoiceDiscount contains one discount applied to an invoice.
type InvoiceDiscount struct {
	DiscountPercentOff *float64 `json:"discount_percent_off,omitempty"`
	DiscountAmountOff  *float64 `json:"discount_amount_off,omitempty"`
}

// PendingChange contains a pending subscription change or cancellation.
type PendingChange struct {
	Kind              string `json:"kind,omitempty"`
	NextTier          string `json:"next_tier,omitempty"`
	NextBillingPeriod string `json:"next_billing_period,omitempty"`
	TimestampSeconds  int64  `json:"timestamp_seconds,omitempty"`
}
