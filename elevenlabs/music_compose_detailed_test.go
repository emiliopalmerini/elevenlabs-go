package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"sync/atomic"
	"testing"
)

func TestComposeDetailedMusicSendsJSONAndParsesMultipartResponse(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.RequestURI() != "/v1/music/detailed?output_format=mp3_44100_192" {
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
		if body["with_timestamps"] != true {
			t.Fatalf("with_timestamps = %#v, want true", body["with_timestamps"])
		}
		if _, ok := body["output_format"]; ok {
			t.Fatal("output_format was present in JSON body, want query-only field")
		}

		metadata := `{
			"composition_plan": {
				"positive_global_styles": ["pop", "rock"],
				"negative_global_styles": ["metal"],
				"sections": [
					{
						"section_name": "Verse 1",
						"positive_local_styles": ["pop"],
						"negative_local_styles": ["metal"],
						"duration_ms": 10000,
						"lines": ["Verse 1 lyrics"]
					}
				]
			},
			"song_metadata": {
				"title": "My Song",
				"description": "My Song Description",
				"genres": ["pop", "rock"],
				"languages": ["en", "fr"],
				"is_explicit": false
			}
		}`
		bodyBytes, contentType := createDetailedMusicMultipart(t, metadata, []byte("detailed-music-audio"))

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("song-id", "song_detailed_123")
		_, _ = w.Write(bodyBytes)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Music.ComposeDetailedWithResponse(ctx, ComposeDetailedMusicRequest{
		ForceInstrumental:       boolPtr(false),
		ModelID:                 MusicModelV2,
		MusicLengthMS:           intPtr(10_000),
		OutputFormat:            OutputFormatMP3_44100_192,
		Prompt:                  "A cinematic synth pop track",
		RespectSectionDurations: boolPtr(false),
		Seed:                    intPtr(1234),
		SignWithC2PA:            boolPtr(true),
		StoreForInpainting:      boolPtr(true),
		WithTimestamps:          boolPtr(true),
	})
	if err != nil {
		t.Fatalf("ComposeDetailedWithResponse returned error: %v", err)
	}
	if string(resp.Data.Audio) != "detailed-music-audio" {
		t.Fatalf("Audio = %q, want detailed-music-audio", string(resp.Data.Audio))
	}
	if resp.Data.SongID != "song_detailed_123" {
		t.Fatalf("SongID = %q, want song_detailed_123", resp.Data.SongID)
	}
	if resp.RawResponse.Header.Get("song-id") != "song_detailed_123" {
		t.Fatalf("raw song-id = %q, want song_detailed_123", resp.RawResponse.Header.Get("song-id"))
	}
	if resp.Data.SongMetadata == nil || resp.Data.SongMetadata.Title != "My Song" {
		t.Fatalf("SongMetadata = %#v, want title My Song", resp.Data.SongMetadata)
	}
	if len(resp.Data.SongMetadata.Genres) != 2 || resp.Data.SongMetadata.Genres[1] != "rock" {
		t.Fatalf("SongMetadata.Genres = %#v, want pop/rock", resp.Data.SongMetadata.Genres)
	}

	plan, ok := resp.Data.CompositionPlan.(MusicPrompt)
	if !ok {
		t.Fatalf("CompositionPlan type = %T, want MusicPrompt", resp.Data.CompositionPlan)
	}
	if len(plan.Sections) != 1 || plan.Sections[0].SectionName != "Verse 1" {
		t.Fatalf("CompositionPlan sections = %#v, want Verse 1", plan.Sections)
	}
}

func TestComposeDetailedMusicValidatesDocumentedBounds(t *testing.T) {
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
		in   ComposeDetailedMusicRequest
		want string
	}{
		{
			name: "prompt max length",
			in:   ComposeDetailedMusicRequest{Prompt: strings.Repeat("a", 4101)},
			want: "prompt must be 4100 characters or fewer",
		},
		{
			name: "music length below minimum",
			in:   ComposeDetailedMusicRequest{MusicLengthMS: intPtr(2999)},
			want: "music_length_ms must be between 3000 and 600000",
		},
		{
			name: "music length above maximum",
			in:   ComposeDetailedMusicRequest{MusicLengthMS: intPtr(600001)},
			want: "music_length_ms must be between 3000 and 600000",
		},
		{
			name: "seed below minimum",
			in:   ComposeDetailedMusicRequest{Seed: intPtr(-1)},
			want: "seed must be between 0 and 2147483647",
		},
		{
			name: "seed above maximum",
			in:   ComposeDetailedMusicRequest{Seed: intPtr(2147483648)},
			want: "seed must be between 0 and 2147483647",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Music.ComposeDetailed(ctx, tt.in)
			if err == nil {
				t.Fatal("ComposeDetailed error = nil, want validation error")
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

func createDetailedMusicMultipart(t *testing.T, metadata string, audio []byte) ([]byte, string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	jsonPart, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json"}})
	if err != nil {
		t.Fatalf("create JSON part: %v", err)
	}
	if _, err := jsonPart.Write([]byte(metadata)); err != nil {
		t.Fatalf("write JSON part: %v", err)
	}

	audioPart, err := writer.CreatePart(textproto.MIMEHeader{"Content-Type": {"audio/mpeg"}})
	if err != nil {
		t.Fatalf("create audio part: %v", err)
	}
	if _, err := audioPart.Write(audio); err != nil {
		t.Fatalf("write audio part: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return buf.Bytes(), "multipart/mixed; boundary=" + writer.Boundary()
}
