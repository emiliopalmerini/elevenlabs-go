package elevenlabs

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/websocket"
)

func TestRealtimeTranscriptSessionSendsConfigAndAudio(t *testing.T) {
	ctx := context.Background()

	var sawRequest bool
	server := httptest.NewServer(websocket.Server{
		Handshake: func(_ *websocket.Config, r *http.Request) error {
			sawRequest = true
			if r.URL.Path != "/v1/speech-to-text/realtime" {
				t.Fatalf("path = %s, want /v1/speech-to-text/realtime", r.URL.Path)
			}
			query := r.URL.Query()
			if query.Get("model_id") != "scribe_v2" {
				t.Fatalf("model_id = %q, want scribe_v2", query.Get("model_id"))
			}
			if query.Get("include_timestamps") != "true" {
				t.Fatalf("include_timestamps = %q, want true", query.Get("include_timestamps"))
			}
			if query.Get("include_language_detection") != "true" {
				t.Fatalf("include_language_detection = %q, want true", query.Get("include_language_detection"))
			}
			if query.Get("audio_format") != "pcm_16000" {
				t.Fatalf("audio_format = %q, want pcm_16000", query.Get("audio_format"))
			}
			if query.Get("language_code") != "en" {
				t.Fatalf("language_code = %q, want en", query.Get("language_code"))
			}
			if query.Get("commit_strategy") != "manual" {
				t.Fatalf("commit_strategy = %q, want manual", query.Get("commit_strategy"))
			}
			assertQueryValues(t, query["keyterms"], []string{"ElevenLabs", "Scribe"})
			if query.Get("no_verbatim") != "true" {
				t.Fatalf("no_verbatim = %q, want true", query.Get("no_verbatim"))
			}
			if query.Get("vad_silence_threshold_secs") != "1.25" {
				t.Fatalf("vad_silence_threshold_secs = %q, want 1.25", query.Get("vad_silence_threshold_secs"))
			}
			if query.Get("vad_threshold") != "0.45" {
				t.Fatalf("vad_threshold = %q, want 0.45", query.Get("vad_threshold"))
			}
			if query.Get("min_speech_duration_ms") != "120" {
				t.Fatalf("min_speech_duration_ms = %q, want 120", query.Get("min_speech_duration_ms"))
			}
			if query.Get("min_silence_duration_ms") != "180" {
				t.Fatalf("min_silence_duration_ms = %q, want 180", query.Get("min_silence_duration_ms"))
			}
			if query.Get("enable_logging") != "false" {
				t.Fatalf("enable_logging = %q, want false", query.Get("enable_logging"))
			}
			if got := r.Header.Get("xi-api-key"); got != "test-key" {
				t.Fatalf("xi-api-key = %q, want test-key", got)
			}
			return nil
		},
		Handler: func(ws *websocket.Conn) {
			if err := websocket.JSON.Send(ws, RealtimeTranscriptEvent{
				MessageType: "session_started",
				SessionID:   "sess_123",
			}); err != nil {
				t.Errorf("send session_started: %v", err)
				return
			}

			var chunk realtimeAudioChunkMessage
			if err := websocket.JSON.Receive(ws, &chunk); err != nil {
				t.Errorf("receive audio chunk: %v", err)
				return
			}
			if chunk.MessageType != "input_audio_chunk" {
				t.Errorf("chunk message_type = %q, want input_audio_chunk", chunk.MessageType)
			}
			if chunk.AudioBase64 != base64.StdEncoding.EncodeToString([]byte("audio")) {
				t.Errorf("chunk audio_base_64 = %q, want encoded audio", chunk.AudioBase64)
			}
			if !chunk.Commit {
				t.Error("chunk commit = false, want true")
			}
			if chunk.SampleRate != 16000 {
				t.Errorf("chunk sample_rate = %d, want 16000", chunk.SampleRate)
			}
			if chunk.PreviousText != "previous context" {
				t.Errorf("chunk previous_text = %q, want previous context", chunk.PreviousText)
			}

			_ = websocket.JSON.Send(ws, RealtimeTranscriptEvent{
				MessageType: "committed_transcript",
				Text:        "hello world",
			})
		},
	})
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	session, err := client.STT.ConnectRealtimeTranscript(ctx, RealtimeTranscriptRequest{
		ModelID:                  "scribe_v2",
		IncludeTimestamps:        boolPtr(true),
		IncludeLanguageDetection: boolPtr(true),
		AudioFormat:              "pcm_16000",
		LanguageCode:             "en",
		CommitStrategy:           "manual",
		Keyterms:                 []string{"ElevenLabs", "Scribe"},
		NoVerbatim:               boolPtr(true),
		VADSilenceThresholdSecs:  floatPtr(1.25),
		VADThreshold:             floatPtr(0.45),
		MinSpeechDurationMS:      intPtr(120),
		MinSilenceDurationMS:     intPtr(180),
		EnableLogging:            boolPtr(false),
	})
	if err != nil {
		t.Fatalf("ConnectRealtimeTranscript returned error: %v", err)
	}
	defer session.Close()

	started, err := session.Receive()
	if err != nil {
		t.Fatalf("Receive session_started returned error: %v", err)
	}
	if started.MessageType != "session_started" || started.SessionID != "sess_123" {
		t.Fatalf("started event = %+v, want session_started sess_123", started)
	}

	if err := session.SendAudioChunk(RealtimeAudioChunk{
		Audio:        []byte("audio"),
		Commit:       true,
		SampleRate:   16000,
		PreviousText: "previous context",
	}); err != nil {
		t.Fatalf("SendAudioChunk returned error: %v", err)
	}

	committed, err := session.Receive()
	if err != nil {
		t.Fatalf("Receive committed_transcript returned error: %v", err)
	}
	if committed.MessageType != "committed_transcript" || committed.Text != "hello world" {
		t.Fatalf("committed event = %+v, want committed transcript", committed)
	}
	if !sawRequest {
		t.Fatal("websocket server did not see handshake request")
	}
}

func TestRealtimeTranscriptSessionAllowsTokenAuth(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(websocket.Server{
		Handshake: func(_ *websocket.Config, r *http.Request) error {
			if r.URL.Query().Get("token") != "token_123" {
				t.Fatalf("token = %q, want token_123", r.URL.Query().Get("token"))
			}
			if got := r.Header.Get("xi-api-key"); got != "" {
				t.Fatalf("xi-api-key = %q, want empty when token auth is used", got)
			}
			return nil
		},
		Handler: func(ws *websocket.Conn) {
			_ = websocket.JSON.Send(ws, RealtimeTranscriptEvent{MessageType: "session_started"})
		},
	})
	defer server.Close()

	client := NewClient("", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	session, err := client.STT.ConnectRealtimeTranscript(ctx, RealtimeTranscriptRequest{
		ModelID: "scribe_v2",
		Token:   "token_123",
	})
	if err != nil {
		t.Fatalf("ConnectRealtimeTranscript returned error: %v", err)
	}
	defer session.Close()
}

func assertQueryValues(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("query values = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("query values = %v, want %v", got, want)
		}
	}
}
