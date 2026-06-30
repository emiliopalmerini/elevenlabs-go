package elevenlabs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCreateCompositionPlanSendsJSONAndParsesMusicPrompt(t *testing.T) {
	ctx := context.Background()

	sourcePlan := MusicPrompt{
		NegativeGlobalStyles: []string{"metal"},
		PositiveGlobalStyles: []string{"pop"},
		Sections: []SongSection{
			{
				DurationMS:          8000,
				Lines:               []string{"source lyrics"},
				NegativeLocalStyles: []string{"distorted"},
				PositiveLocalStyles: []string{"bright"},
				SectionName:         "Verse 1",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.RequestURI() != "/v1/music/plan" {
			t.Fatalf("request uri = %s, want /v1/music/plan", r.URL.RequestURI())
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["prompt"] != "A compact pop-rock chorus plan" {
			t.Fatalf("prompt = %q, want prompt", body["prompt"])
		}
		if body["model_id"] != string(MusicModelV1) {
			t.Fatalf("model_id = %q, want %s", body["model_id"], MusicModelV1)
		}
		if body["music_length_ms"] != float64(10_000) {
			t.Fatalf("music_length_ms = %#v, want 10000", body["music_length_ms"])
		}
		source, ok := body["source_composition_plan"].(map[string]any)
		if !ok {
			t.Fatalf("source_composition_plan = %#v, want object", body["source_composition_plan"])
		}
		sections, ok := source["sections"].([]any)
		if !ok || len(sections) != 1 {
			t.Fatalf("source sections = %#v, want one section", source["sections"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"negative_global_styles": ["metal", "hip-hop", "country"],
			"positive_global_styles": ["pop", "rock", "jazz"],
			"sections": [
				{
					"duration_ms": 10000,
					"lines": ["Verse 1 lyrics"],
					"negative_local_styles": ["metal"],
					"positive_local_styles": ["pop"],
					"section_name": "Verse 1"
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Music.CreateCompositionPlanWithResponse(ctx, CreateCompositionPlanRequest{
		ModelID:               MusicModelV1,
		MusicLengthMS:         intPtr(10_000),
		Prompt:                "A compact pop-rock chorus plan",
		SourceCompositionPlan: sourcePlan,
	})
	if err != nil {
		t.Fatalf("CreateCompositionPlanWithResponse returned error: %v", err)
	}
	if resp.RawResponse.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.RawResponse.StatusCode, http.StatusOK)
	}

	plan, ok := resp.Data.(MusicPrompt)
	if !ok {
		t.Fatalf("Data type = %T, want MusicPrompt", resp.Data)
	}
	if len(plan.PositiveGlobalStyles) != 3 || plan.PositiveGlobalStyles[0] != "pop" {
		t.Fatalf("PositiveGlobalStyles = %#v, want pop/rock/jazz", plan.PositiveGlobalStyles)
	}
	if len(plan.Sections) != 1 || plan.Sections[0].SectionName != "Verse 1" {
		t.Fatalf("Sections = %#v, want Verse 1", plan.Sections)
	}
}

func TestCreateCompositionPlanParsesV2CompositionPlan(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.RequestURI() != "/v1/music/plan" {
			t.Fatalf("request uri = %s, want /v1/music/plan", r.URL.RequestURI())
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"chunks": [
				{
					"condition_strength": "medium",
					"conditioning_ref": {
						"range": {"end_ms": 5000, "start_ms": 0},
						"song_id": "song_ref"
					},
					"context_adherence": "high",
					"duration_ms": 10000,
					"negative_styles": ["metal"],
					"positive_styles": ["ambient pop"],
					"text": "[Verse 1]\nLine one"
				},
				{
					"range": {"end_ms": 9000, "start_ms": 5000},
					"song_id": "song_existing"
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	out, err := client.Music.CreateCompositionPlan(ctx, CreateCompositionPlanRequest{
		ModelID: MusicModelV2,
		Prompt:  "Create a v2 plan",
	})
	if err != nil {
		t.Fatalf("CreateCompositionPlan returned error: %v", err)
	}

	plan, ok := out.(CompositionPlan)
	if !ok {
		t.Fatalf("Data type = %T, want CompositionPlan", out)
	}
	if len(plan.Chunks) != 2 {
		t.Fatalf("Chunks length = %d, want 2", len(plan.Chunks))
	}

	generated, ok := plan.Chunks[0].(GenerationChunkInput)
	if !ok {
		t.Fatalf("Chunks[0] type = %T, want GenerationChunkInput", plan.Chunks[0])
	}
	if generated.ConditionStrength != "medium" {
		t.Fatalf("ConditionStrength = %q, want medium", generated.ConditionStrength)
	}
	if generated.ConditioningRef == nil || generated.ConditioningRef.SongID != "song_ref" {
		t.Fatalf("ConditioningRef = %#v, want song_ref", generated.ConditioningRef)
	}
	if generated.Text != "[Verse 1]\nLine one" {
		t.Fatalf("Text = %q, want verse text", generated.Text)
	}

	audioRef, ok := plan.Chunks[1].(AudioRefChunk)
	if !ok {
		t.Fatalf("Chunks[1] type = %T, want AudioRefChunk", plan.Chunks[1])
	}
	if audioRef.SongID != "song_existing" || audioRef.Range.StartMS != 5000 {
		t.Fatalf("AudioRefChunk = %#v, want song_existing starting at 5000", audioRef)
	}
}

func TestCreateCompositionPlanValidatesDocumentedFields(t *testing.T) {
	ctx := context.Background()
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"positive_global_styles":[],"negative_global_styles":[],"sections":[]}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	tests := []struct {
		name string
		in   CreateCompositionPlanRequest
		want string
	}{
		{
			name: "prompt required",
			in:   CreateCompositionPlanRequest{},
			want: "prompt is required",
		},
		{
			name: "prompt cannot be blank",
			in:   CreateCompositionPlanRequest{Prompt: "   "},
			want: "prompt is required",
		},
		{
			name: "music length below minimum",
			in:   CreateCompositionPlanRequest{MusicLengthMS: intPtr(2999), Prompt: "A plan"},
			want: "music_length_ms must be between 3000 and 600000",
		},
		{
			name: "music length above maximum",
			in:   CreateCompositionPlanRequest{MusicLengthMS: intPtr(600001), Prompt: "A plan"},
			want: "music_length_ms must be between 3000 and 600000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Music.CreateCompositionPlan(ctx, tt.in)
			if err == nil {
				t.Fatal("CreateCompositionPlan error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tt.want)
			}
		})
	}

	if requests.Load() != 0 {
		t.Fatalf("server requests = %d, want 0 for validation failures", requests.Load())
	}
}

func TestCreateCompositionPlanReturnsValidationAPIError(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":[{"loc":["body","prompt"],"msg":"Field required","type":"missing"}]}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithoutRetries())

	_, err := client.Music.CreateCompositionPlan(ctx, CreateCompositionPlanRequest{Prompt: "A plan"})
	if err == nil {
		t.Fatal("CreateCompositionPlan error = nil, want API error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnprocessableEntity)
	}
	if len(apiErr.Validation) != 1 {
		t.Fatalf("Validation length = %d, want 1", len(apiErr.Validation))
	}
	validation := apiErr.Validation[0]
	if validation.Msg != "Field required" {
		t.Fatalf("Validation Msg = %q, want Field required", validation.Msg)
	}
	if validation.Type != "missing" {
		t.Fatalf("Validation Type = %q, want missing", validation.Type)
	}
	if len(validation.Loc) != 2 || validation.Loc[0] != "body" || validation.Loc[1] != "prompt" {
		t.Fatalf("Validation Loc = %#v, want body.prompt", validation.Loc)
	}
}
