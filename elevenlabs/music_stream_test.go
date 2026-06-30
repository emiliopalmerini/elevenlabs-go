package elevenlabs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestStreamMusicSendsJSONAndReturnsStream(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.RequestURI() != "/v1/music/stream?output_format=mp3_44100_192" {
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
		if body["store_for_inpainting"] != true {
			t.Fatalf("store_for_inpainting = %#v, want true", body["store_for_inpainting"])
		}
		if _, ok := body["output_format"]; ok {
			t.Fatal("output_format was present in JSON body, want query-only field")
		}
		if _, ok := body["respect_sections_durations"]; ok {
			t.Fatal("respect_sections_durations was present in stream body, want compose-only field omitted")
		}
		if _, ok := body["sign_with_c2pa"]; ok {
			t.Fatal("sign_with_c2pa was present in stream body, want compose-only field omitted")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("song-id", "song_stream_123")
		_, _ = w.Write([]byte("streamed-music-audio"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.Music.StreamWithResponse(ctx, StreamMusicRequest{
		ForceInstrumental:  boolPtr(false),
		ModelID:            MusicModelV2,
		MusicLengthMS:      intPtr(10_000),
		OutputFormat:       OutputFormatMP3_44100_192,
		Prompt:             "A cinematic synth pop track",
		Seed:               intPtr(1234),
		StoreForInpainting: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("StreamWithResponse returned error: %v", err)
	}
	defer resp.Data.Close()

	body, err := io.ReadAll(resp.Data)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if string(body) != "streamed-music-audio" {
		t.Fatalf("stream body = %q, want streamed-music-audio", string(body))
	}
	if resp.Data.SongID != "song_stream_123" {
		t.Fatalf("SongID = %q, want song_stream_123", resp.Data.SongID)
	}
	if resp.RawResponse.Header.Get("song-id") != "song_stream_123" {
		t.Fatalf("raw song-id = %q, want song_stream_123", resp.RawResponse.Header.Get("song-id"))
	}
}

func TestStreamMusicValidatesDocumentedBounds(t *testing.T) {
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
		in   StreamMusicRequest
		want string
	}{
		{
			name: "prompt max length",
			in:   StreamMusicRequest{Prompt: strings.Repeat("a", 4101)},
			want: "prompt must be 4100 characters or fewer",
		},
		{
			name: "music length below minimum",
			in:   StreamMusicRequest{MusicLengthMS: intPtr(2999)},
			want: "music_length_ms must be between 3000 and 600000",
		},
		{
			name: "music length above maximum",
			in:   StreamMusicRequest{MusicLengthMS: intPtr(600001)},
			want: "music_length_ms must be between 3000 and 600000",
		},
		{
			name: "seed below minimum",
			in:   StreamMusicRequest{Seed: intPtr(-1)},
			want: "seed must be between 0 and 2147483647",
		},
		{
			name: "seed above maximum",
			in:   StreamMusicRequest{Seed: intPtr(2147483648)},
			want: "seed must be between 0 and 2147483647",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Music.Stream(ctx, tt.in)
			if err == nil {
				t.Fatal("Stream error = nil, want validation error")
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
