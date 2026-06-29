package elevenlabs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCreateSpeechSendsJSONAndReturnsAudio(t *testing.T) {
	ctx := context.Background()
	stability := 0.7
	speed := 1.1
	useSpeakerBoost := false
	seed := 1234
	usePVC := true
	applyLanguageNormalization := true
	enableLogging := false
	latency := 2

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.RequestURI() != "/v1/text-to-speech/voice%2Fid?enable_logging=false&optimize_streaming_latency=2&output_format=mp3_44100_128" {
			t.Fatalf("request uri = %s, want escaped voice id and query", r.URL.RequestURI())
		}
		if got := r.Header.Get("xi-api-key"); got != "test-key" {
			t.Fatalf("xi-api-key = %q, want test-key", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}

		var body CreateSpeechRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.VoiceID != "" {
			t.Fatalf("VoiceID in body = %q, want omitted", body.VoiceID)
		}
		if body.Text != "Hello from Go." {
			t.Fatalf("Text = %q, want Hello from Go.", body.Text)
		}
		if body.ModelID != "eleven_multilingual_v2" {
			t.Fatalf("ModelID = %q, want eleven_multilingual_v2", body.ModelID)
		}
		if body.LanguageCode != "en" {
			t.Fatalf("LanguageCode = %q, want en", body.LanguageCode)
		}
		if body.VoiceSettings == nil || body.VoiceSettings.Stability == nil || *body.VoiceSettings.Stability != stability {
			t.Fatalf("VoiceSettings.Stability = %#v, want %f", body.VoiceSettings, stability)
		}
		if body.VoiceSettings.UseSpeakerBoost == nil || *body.VoiceSettings.UseSpeakerBoost {
			t.Fatalf("UseSpeakerBoost = %#v, want false", body.VoiceSettings.UseSpeakerBoost)
		}
		if body.VoiceSettings.Speed == nil || *body.VoiceSettings.Speed != speed {
			t.Fatalf("Speed = %#v, want %f", body.VoiceSettings.Speed, speed)
		}
		if len(body.PronunciationDictionaryLocators) != 1 || body.PronunciationDictionaryLocators[0].PronunciationDictionaryID != "dict_123" {
			t.Fatalf("PronunciationDictionaryLocators = %#v, want dict_123", body.PronunciationDictionaryLocators)
		}
		if body.Seed == nil || *body.Seed != seed {
			t.Fatalf("Seed = %#v, want %d", body.Seed, seed)
		}
		if body.PreviousText != "Previous sentence." {
			t.Fatalf("PreviousText = %q, want Previous sentence.", body.PreviousText)
		}
		if body.NextText != "Next sentence." {
			t.Fatalf("NextText = %q, want Next sentence.", body.NextText)
		}
		assertStringSlice(t, body.PreviousRequestIDs, []string{"prev_req"})
		assertStringSlice(t, body.NextRequestIDs, []string{"next_req"})
		if body.UsePVCAsIVC == nil || !*body.UsePVCAsIVC {
			t.Fatalf("UsePVCAsIVC = %#v, want true", body.UsePVCAsIVC)
		}
		if body.ApplyTextNormalization != "off" {
			t.Fatalf("ApplyTextNormalization = %q, want off", body.ApplyTextNormalization)
		}
		if body.ApplyLanguageTextNormalization == nil || !*body.ApplyLanguageTextNormalization {
			t.Fatalf("ApplyLanguageTextNormalization = %#v, want true", body.ApplyLanguageTextNormalization)
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Request-ID", "req_123")
		_, _ = w.Write([]byte("audio-bytes"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.TTS.CreateSpeechWithResponse(ctx, CreateSpeechRequest{
		VoiceID:      "voice/id",
		Text:         "Hello from Go.",
		ModelID:      "eleven_multilingual_v2",
		LanguageCode: "en",
		VoiceSettings: &VoiceSettings{
			Stability:       &stability,
			UseSpeakerBoost: &useSpeakerBoost,
			Speed:           &speed,
		},
		PronunciationDictionaryLocators: []PronunciationDictionaryLocator{
			{PronunciationDictionaryID: "dict_123", VersionID: "version_1"},
		},
		Seed:                           &seed,
		PreviousText:                   "Previous sentence.",
		NextText:                       "Next sentence.",
		PreviousRequestIDs:             []string{"prev_req"},
		NextRequestIDs:                 []string{"next_req"},
		UsePVCAsIVC:                    &usePVC,
		ApplyTextNormalization:         "off",
		ApplyLanguageTextNormalization: &applyLanguageNormalization,
		EnableLogging:                  &enableLogging,
		OptimizeStreamingLatency:       &latency,
		OutputFormat:                   "mp3_44100_128",
	})
	if err != nil {
		t.Fatalf("CreateSpeechWithResponse returned error: %v", err)
	}
	if string(resp.Data) != "audio-bytes" {
		t.Fatalf("Data = %q, want audio-bytes", string(resp.Data))
	}
	if resp.RawResponse.Header.Get("X-Request-ID") != "req_123" {
		t.Fatalf("X-Request-ID = %q, want req_123", resp.RawResponse.Header.Get("X-Request-ID"))
	}
}

func TestCreateSpeechWithTimestampsParsesAlignmentAndAudio(t *testing.T) {
	ctx := context.Background()
	audio := base64.StdEncoding.EncodeToString([]byte("audio"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/text-to-speech/voice_123/with-timestamps" {
			t.Fatalf("path = %s, want with-timestamps endpoint", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"audio_base64": "` + audio + `",
			"alignment": {
				"characters": ["H", "i"],
				"character_start_times_seconds": [0, 0.1],
				"character_end_times_seconds": [0.1, 0.2]
			},
			"normalized_alignment": {
				"characters": ["H", "i"],
				"character_start_times_seconds": [0, 0.1],
				"character_end_times_seconds": [0.1, 0.2]
			}
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	out, err := client.TTS.CreateSpeechWithTimestamps(ctx, CreateSpeechRequest{
		VoiceID: "voice_123",
		Text:    "Hi",
	})
	if err != nil {
		t.Fatalf("CreateSpeechWithTimestamps returned error: %v", err)
	}
	decoded, err := out.Audio()
	if err != nil {
		t.Fatalf("Audio returned error: %v", err)
	}
	if string(decoded) != "audio" {
		t.Fatalf("Audio = %q, want audio", string(decoded))
	}
	if out.Alignment == nil || len(out.Alignment.Characters) != 2 || out.Alignment.Characters[1] != "i" {
		t.Fatalf("Alignment = %#v, want Hi alignment", out.Alignment)
	}
}

func TestStreamSpeechReturnsReadableBodyAndRawMetadata(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/text-to-speech/voice_123/stream" {
			t.Fatalf("path = %s, want stream endpoint", r.URL.Path)
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("X-Request-ID", "req_stream")
		_, _ = w.Write([]byte("streamed-audio"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	resp, err := client.TTS.StreamSpeechWithResponse(ctx, CreateSpeechRequest{
		VoiceID: "voice_123",
		Text:    "stream me",
	})
	if err != nil {
		t.Fatalf("StreamSpeechWithResponse returned error: %v", err)
	}
	defer resp.Data.Close()

	body, err := io.ReadAll(resp.Data)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if string(body) != "streamed-audio" {
		t.Fatalf("stream body = %q, want streamed-audio", string(body))
	}
	if resp.RawResponse.Header.Get("X-Request-ID") != "req_stream" {
		t.Fatalf("X-Request-ID = %q, want req_stream", resp.RawResponse.Header.Get("X-Request-ID"))
	}
}

func TestStreamSpeechWithTimestampsReceivesChunks(t *testing.T) {
	ctx := context.Background()
	firstAudio := base64.StdEncoding.EncodeToString([]byte("first"))
	secondAudio := base64.StdEncoding.EncodeToString([]byte("second"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/text-to-speech/voice_123/stream/with-timestamps" {
			t.Fatalf("path = %s, want stream with timestamps endpoint", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"audio_base64":"` + firstAudio + `","alignment":{"characters":["A"],"character_start_times_seconds":[0],"character_end_times_seconds":[0.1]}}` + "\n\n"))
		_, _ = w.Write([]byte(`{"audio_base64":"` + secondAudio + `"}` + "\n"))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	stream, err := client.TTS.StreamSpeechWithTimestamps(ctx, CreateSpeechRequest{
		VoiceID: "voice_123",
		Text:    "stream with timing",
	})
	if err != nil {
		t.Fatalf("StreamSpeechWithTimestamps returned error: %v", err)
	}
	defer stream.Close()

	first, err := stream.Receive()
	if err != nil {
		t.Fatalf("Receive first chunk returned error: %v", err)
	}
	decodedFirst, err := first.Audio()
	if err != nil {
		t.Fatalf("decode first audio: %v", err)
	}
	if string(decodedFirst) != "first" {
		t.Fatalf("first audio = %q, want first", string(decodedFirst))
	}
	if first.Alignment == nil || first.Alignment.Characters[0] != "A" {
		t.Fatalf("first alignment = %#v, want A", first.Alignment)
	}

	second, err := stream.Receive()
	if err != nil {
		t.Fatalf("Receive second chunk returned error: %v", err)
	}
	decodedSecond, err := second.Audio()
	if err != nil {
		t.Fatalf("decode second audio: %v", err)
	}
	if string(decodedSecond) != "second" {
		t.Fatalf("second audio = %q, want second", string(decodedSecond))
	}

	if _, err := stream.Receive(); !errors.Is(err, io.EOF) {
		t.Fatalf("Receive EOF error = %v, want io.EOF", err)
	}
}

func TestStreamSpeechRetriesBeforeReturningBody(t *testing.T) {
	ctx := context.Background()
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt == 1 {
			http.Error(w, `{"detail":{"message":"temporary"}}`, http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("after-retry"))
	}))
	defer server.Close()

	client := NewClient(
		"test-key",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRetryConfig(RetryConfig{
			MaxAttempts: 2,
			BaseDelay:   time.Millisecond,
			MaxDelay:    time.Millisecond,
		}),
	)

	stream, err := client.TTS.StreamSpeech(ctx, CreateSpeechRequest{
		VoiceID: "voice_123",
		Text:    "retry stream",
	})
	if err != nil {
		t.Fatalf("StreamSpeech returned error: %v", err)
	}
	defer stream.Close()

	body, err := io.ReadAll(stream)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if string(body) != "after-retry" {
		t.Fatalf("body = %q, want after-retry", string(body))
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestCreateSpeechValidatesRequiredFields(t *testing.T) {
	ctx := context.Background()
	client := NewClient("test-key")

	tests := []struct {
		name string
		in   CreateSpeechRequest
		want string
	}{
		{
			name: "voice id",
			in:   CreateSpeechRequest{Text: "hello"},
			want: "voice_id is required",
		},
		{
			name: "text",
			in:   CreateSpeechRequest{VoiceID: "voice_123"},
			want: "text is required",
		},
		{
			name: "latency",
			in: CreateSpeechRequest{
				VoiceID:                  "voice_123",
				Text:                     "hello",
				OptimizeStreamingLatency: intPtr(5),
			},
			want: "optimize_streaming_latency must be between 0 and 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.TTS.CreateSpeech(ctx, tt.in)
			if err == nil {
				t.Fatal("CreateSpeech error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tt.want)
			}
		})
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("slice = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
