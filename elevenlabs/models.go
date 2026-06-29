package elevenlabs

import (
	"context"
	"errors"
)

// ModelsService provides ElevenLabs model APIs.
type ModelsService struct {
	client *Client
}

// Model contains metadata for an ElevenLabs model.
type Model struct {
	ModelID                            string          `json:"model_id"`
	Name                               string          `json:"name,omitempty"`
	CanBeFinetuned                     bool            `json:"can_be_finetuned,omitempty"`
	CanDoTextToSpeech                  bool            `json:"can_do_text_to_speech,omitempty"`
	CanDoVoiceConversion               bool            `json:"can_do_voice_conversion,omitempty"`
	CanUseStyle                        bool            `json:"can_use_style,omitempty"`
	CanUseSpeakerBoost                 bool            `json:"can_use_speaker_boost,omitempty"`
	ServesProVoices                    bool            `json:"serves_pro_voices,omitempty"`
	TokenCostFactor                    float64         `json:"token_cost_factor,omitempty"`
	Description                        string          `json:"description,omitempty"`
	RequiresAlphaAccess                bool            `json:"requires_alpha_access,omitempty"`
	MaxCharactersRequestFreeUser       int             `json:"max_characters_request_free_user,omitempty"`
	MaxCharactersRequestSubscribedUser int             `json:"max_characters_request_subscribed_user,omitempty"`
	MaximumTextLengthPerRequest        int             `json:"maximum_text_length_per_request,omitempty"`
	Languages                          []ModelLanguage `json:"languages,omitempty"`
	ModelRates                         *ModelRates     `json:"model_rates,omitempty"`
	ConcurrencyGroup                   string          `json:"concurrency_group,omitempty"`
}

// ModelLanguage contains language metadata supported by a model.
type ModelLanguage struct {
	LanguageID string `json:"language_id"`
	Name       string `json:"name"`
}

// ModelRates contains billing rate metadata for a model.
type ModelRates struct {
	CharacterCostMultiplier float64 `json:"character_cost_multiplier"`
	CostDiscountMultiplier  float64 `json:"cost_discount_multiplier,omitempty"`
}

// List gets the list of available models.
func (s *ModelsService) List(ctx context.Context) ([]Model, error) {
	resp, err := s.ListWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ListWithResponse gets the list of available models and returns HTTP response
// metadata.
func (s *ModelsService) ListWithResponse(ctx context.Context) (*Response[[]Model], error) {
	var out []Model
	client, err := s.apiClient()
	if err != nil {
		return nil, err
	}
	raw, err := client.getJSON(ctx, "/v1/models", &out)
	if err != nil {
		return nil, err
	}

	return &Response[[]Model]{
		Data:        out,
		RawResponse: raw,
	}, nil
}

func (s *ModelsService) apiClient() (*Client, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	return s.client, nil
}
