package elevenlabs

import (
	"context"
	"net/http"
)

// ModelsService provides model API methods.
type ModelsService struct {
	client *Client
}

type Model struct {
	ModelID                            string      `json:"model_id"`
	Name                               string      `json:"name,omitempty"`
	Description                        string      `json:"description,omitempty"`
	CanBeFinetuned                     bool        `json:"can_be_finetuned,omitempty"`
	CanDoTextToSpeech                  bool        `json:"can_do_text_to_speech,omitempty"`
	CanDoVoiceConversion               bool        `json:"can_do_voice_conversion,omitempty"`
	CanUseStyle                        bool        `json:"can_use_style,omitempty"`
	CanUseSpeakerBoost                 bool        `json:"can_use_speaker_boost,omitempty"`
	ServesProVoices                    bool        `json:"serves_pro_voices,omitempty"`
	TokenCostFactor                    float64     `json:"token_cost_factor,omitempty"`
	RequiresAlphaAccess                bool        `json:"requires_alpha_access,omitempty"`
	MaxCharactersRequestFreeUser       int         `json:"max_characters_request_free_user,omitempty"`
	MaxCharactersRequestSubscribedUser int         `json:"max_characters_request_subscribed_user,omitempty"`
	MaximumTextLengthPerRequest        int         `json:"maximum_text_length_per_request,omitempty"`
	Languages                          []Language  `json:"languages,omitempty"`
	ModelRates                         *ModelRates `json:"model_rates,omitempty"`
	ConcurrencyGroup                   string      `json:"concurrency_group,omitempty"`
}

type Language struct {
	LanguageID string `json:"language_id"`
	Name       string `json:"name"`
}

type ModelRates struct {
	CharacterCostMultiplier float64 `json:"character_cost_multiplier"`
	CostDiscountMultiplier  float64 `json:"cost_discount_multiplier,omitempty"`
}

// List fetches available models.
func (s *ModelsService) List(ctx context.Context) ([]Model, error) {
	var out []Model
	path := "/v1/models"
	err := s.client.doJSON(ctx, http.MethodGet, path, nil, true, func(ctx context.Context) (*http.Request, error) {
		return s.client.newRequest(ctx, http.MethodGet, path, nil, nil)
	}, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
