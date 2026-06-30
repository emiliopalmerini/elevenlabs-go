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

func TestComposeMusicSendsJSONAndReturnsComposition(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.RequestURI() != "/v1/music?output_format=mp3_44100_192" {
			t.Fatalf("request uri = %s, want output_format query", r.URL.RequestURI())
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
		if body["prompt"] != "A cinematic synth pop track" {
			t.Fatalf("prompt = %q, want prompt", body["prompt"])
		}
		if body["model_id"] != string(MusicModelV2) {
			t.Fatalf("model_id = %q, want %s", body["model_id"], MusicModelV2)
		}
		if body["music_length_ms"] != float64(10_000) {
			t.Fatalf("music_length_ms = %#v, want 10000", body["music_length_ms"])
		}
		if body["seed"] != float64(1234) {
			t.Fatalf("seed = %#v, want 1234", body["seed"])
		}
		if body["force_instrumental"] != false {
			t.Fatalf("force_instrumental = %#v, want false", body["force_instrumental"])
		}
		if body["respect_sections_durations"] != false {
			t.Fatalf("respect_sections_durations = %#v, want false", body["respect_sections_durations"])
		}
		if body["sign_with_c2pa"] != true {
			t.Fatalf("sign_with_c2pa = %#v, want true", body["sign_with_c2pa"])
		}
		if body["store_for_inpainting"] != true {
			t.Fatalf("store_for_inpainting = %#v, want true", body["store_for_inpainting"])
		}
		if _, ok := body["output_format"]; ok {
			t.Fatal("output_format was present in JSON body, want query-only field")
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("song-id", "song_123")
		_, _ = w.Write([]byte("music-audio"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Music.ComposeWithResponse(ctx, ComposeMusicRequest{
		ForceInstrumental:       boolPtr(false),
		ModelID:                 MusicModelV2,
		MusicLengthMS:           intPtr(10_000),
		OutputFormat:            OutputFormatMP3_44100_192,
		Prompt:                  "A cinematic synth pop track",
		RespectSectionDurations: boolPtr(false),
		Seed:                    intPtr(1234),
		SignWithC2PA:            boolPtr(true),
		StoreForInpainting:      boolPtr(true),
	})
	if err != nil {
		t.Fatalf("ComposeWithResponse returned error: %v", err)
	}
	if string(resp.Data.Audio) != "music-audio" {
		t.Fatalf("Audio = %q, want music-audio", string(resp.Data.Audio))
	}
	if resp.Data.SongID != "song_123" {
		t.Fatalf("SongID = %q, want song_123", resp.Data.SongID)
	}
	if resp.RawResponse.Header.Get("song-id") != "song_123" {
		t.Fatalf("raw song-id = %q, want song_123", resp.RawResponse.Header.Get("song-id"))
	}
}

func TestComposeMusicValidatesDocumentedBounds(t *testing.T) {
	ctx := context.Background()
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("unexpected-audio"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	tests := []struct {
		name string
		in   ComposeMusicRequest
		want string
	}{
		{
			name: "prompt max length",
			in:   ComposeMusicRequest{Prompt: strings.Repeat("a", 4101)},
			want: "prompt must be 4100 characters or fewer",
		},
		{
			name: "music length below minimum",
			in:   ComposeMusicRequest{MusicLengthMS: intPtr(2999)},
			want: "music_length_ms must be between 3000 and 600000",
		},
		{
			name: "music length above maximum",
			in:   ComposeMusicRequest{MusicLengthMS: intPtr(600001)},
			want: "music_length_ms must be between 3000 and 600000",
		},
		{
			name: "seed below minimum",
			in:   ComposeMusicRequest{Seed: intPtr(-1)},
			want: "seed must be between 0 and 2147483647",
		},
		{
			name: "seed above maximum",
			in:   ComposeMusicRequest{Seed: intPtr(2147483648)},
			want: "seed must be between 0 and 2147483647",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Music.Compose(ctx, tt.in)
			if err == nil {
				t.Fatal("Compose error = nil, want validation error")
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

func TestComposeMusicReturnsValidationAPIError(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":[{"loc":["body","composition_plan","chunks",0],"msg":"Field required","type":"missing"}]}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithoutRetries())

	_, err := client.Music.Compose(ctx, ComposeMusicRequest{Prompt: "A short instrumental cue"})
	if err == nil {
		t.Fatal("Compose error = nil, want API error")
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
	if len(validation.Loc) != 4 || validation.Loc[0] != "body" || validation.Loc[1] != "composition_plan" || validation.Loc[2] != "chunks" || validation.Loc[3] != float64(0) {
		t.Fatalf("Validation Loc = %#v, want body.composition_plan.chunks.0", validation.Loc)
	}
	if !strings.Contains(apiErr.Message, "body.composition_plan.chunks.0: Field required") {
		t.Fatalf("Message = %q, want validation location summary", apiErr.Message)
	}
}
