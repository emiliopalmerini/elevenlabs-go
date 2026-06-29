package elevenlabs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListModelsParsesDocumentedFields(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.RequestURI() != "/v1/models" {
			t.Fatalf("request uri = %s, want /v1/models", r.URL.RequestURI())
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"model_id": "eleven_multilingual_v2",
				"name": "Eleven Multilingual v2",
				"can_be_finetuned": true,
				"can_do_text_to_speech": true,
				"can_do_voice_conversion": true,
				"can_use_style": true,
				"can_use_speaker_boost": true,
				"serves_pro_voices": true,
				"token_cost_factor": 1.1,
				"description": "Multilingual text to speech",
				"requires_alpha_access": true,
				"max_characters_request_free_user": 2500,
				"max_characters_request_subscribed_user": 5000,
				"maximum_text_length_per_request": 10000,
				"languages": [
					{
						"language_id": "en",
						"name": "English"
					}
				],
				"model_rates": {
					"character_cost_multiplier": 1,
					"cost_discount_multiplier": 0.8
				},
				"concurrency_group": "standard"
			}
		]`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	models, err := client.Models.List(ctx)
	if err != nil {
		t.Fatalf("ListModels returned error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("models length = %d, want 1", len(models))
	}

	model := models[0]
	if model.ModelID != "eleven_multilingual_v2" {
		t.Fatalf("ModelID = %q, want eleven_multilingual_v2", model.ModelID)
	}
	if model.Name != "Eleven Multilingual v2" {
		t.Fatalf("Name = %q, want Eleven Multilingual v2", model.Name)
	}
	if !model.CanBeFinetuned {
		t.Fatal("CanBeFinetuned = false, want true")
	}
	if !model.CanDoTextToSpeech {
		t.Fatal("CanDoTextToSpeech = false, want true")
	}
	if !model.CanDoVoiceConversion {
		t.Fatal("CanDoVoiceConversion = false, want true")
	}
	if !model.CanUseStyle {
		t.Fatal("CanUseStyle = false, want true")
	}
	if !model.CanUseSpeakerBoost {
		t.Fatal("CanUseSpeakerBoost = false, want true")
	}
	if !model.ServesProVoices {
		t.Fatal("ServesProVoices = false, want true")
	}
	if model.TokenCostFactor != 1.1 {
		t.Fatalf("TokenCostFactor = %v, want 1.1", model.TokenCostFactor)
	}
	if model.Description != "Multilingual text to speech" {
		t.Fatalf("Description = %q, want Multilingual text to speech", model.Description)
	}
	if !model.RequiresAlphaAccess {
		t.Fatal("RequiresAlphaAccess = false, want true")
	}
	if model.MaxCharactersRequestFreeUser != 2500 {
		t.Fatalf("MaxCharactersRequestFreeUser = %d, want 2500", model.MaxCharactersRequestFreeUser)
	}
	if model.MaxCharactersRequestSubscribedUser != 5000 {
		t.Fatalf("MaxCharactersRequestSubscribedUser = %d, want 5000", model.MaxCharactersRequestSubscribedUser)
	}
	if model.MaximumTextLengthPerRequest != 10000 {
		t.Fatalf("MaximumTextLengthPerRequest = %d, want 10000", model.MaximumTextLengthPerRequest)
	}
	if len(model.Languages) != 1 || model.Languages[0].LanguageID != "en" || model.Languages[0].Name != "English" {
		t.Fatalf("Languages = %#v, want English language metadata", model.Languages)
	}
	if model.ModelRates == nil {
		t.Fatal("ModelRates = nil, want rates")
	}
	if model.ModelRates.CharacterCostMultiplier != 1 {
		t.Fatalf("CharacterCostMultiplier = %v, want 1", model.ModelRates.CharacterCostMultiplier)
	}
	if model.ModelRates.CostDiscountMultiplier != 0.8 {
		t.Fatalf("CostDiscountMultiplier = %v, want 0.8", model.ModelRates.CostDiscountMultiplier)
	}
	if model.ConcurrencyGroup != "standard" {
		t.Fatalf("ConcurrencyGroup = %q, want standard", model.ConcurrencyGroup)
	}
}

func TestListModelsWithResponseReturnsRawMetadata(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "req_models")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"model_id":"scribe_v2"}]`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Models.ListWithResponse(ctx)
	if err != nil {
		t.Fatalf("ListModelsWithResponse returned error: %v", err)
	}
	if len(resp.Data) != 1 || resp.Data[0].ModelID != "scribe_v2" {
		t.Fatalf("Data = %#v, want scribe_v2 model", resp.Data)
	}
	if resp.RawResponse.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.RawResponse.StatusCode, http.StatusOK)
	}
	if resp.RawResponse.Header.Get("X-Request-ID") != "req_models" {
		t.Fatalf("X-Request-ID = %q, want req_models", resp.RawResponse.Header.Get("X-Request-ID"))
	}
	if resp.RawResponse.URL != server.URL+"/v1/models" {
		t.Fatalf("URL = %q, want %s/v1/models", resp.RawResponse.URL, server.URL)
	}
}

func TestListModelsRetriesTransientStatus(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, `{"detail":"temporary upstream failure"}`, http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"model_id":"scribe_v2"}]`))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(fastRetryConfig(2)),
	)

	models, err := client.Models.List(ctx)
	if err != nil {
		t.Fatalf("ListModels returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if len(models) != 1 || models[0].ModelID != "scribe_v2" {
		t.Fatalf("models = %#v, want scribe_v2 model", models)
	}
}
