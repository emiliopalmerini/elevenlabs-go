package elevenlabs

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/websocket"
)

func TestStreamInputSessionSendsMessagesAndReceivesAudio(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(websocket.Server{
		Handshake: func(_ *websocket.Config, r *http.Request) error {
			if r.URL.Path != "/v1/text-to-speech/voice_123/stream-input" {
				t.Fatalf("path = %s, want stream-input endpoint", r.URL.Path)
			}
			query := r.URL.Query()
			if query.Get("model_id") != "eleven_flash_v2_5" {
				t.Fatalf("model_id = %q, want eleven_flash_v2_5", query.Get("model_id"))
			}
			if query.Get("language_code") != "en" {
				t.Fatalf("language_code = %q, want en", query.Get("language_code"))
			}
			if query.Get("enable_logging") != "false" {
				t.Fatalf("enable_logging = %q, want false", query.Get("enable_logging"))
			}
			if query.Get("enable_ssml_parsing") != "true" {
				t.Fatalf("enable_ssml_parsing = %q, want true", query.Get("enable_ssml_parsing"))
			}
			if query.Get("output_format") != "mp3_44100_128" {
				t.Fatalf("output_format = %q, want mp3_44100_128", query.Get("output_format"))
			}
			if query.Get("inactivity_timeout") != "30" {
				t.Fatalf("inactivity_timeout = %q, want 30", query.Get("inactivity_timeout"))
			}
			if query.Get("sync_alignment") != "true" {
				t.Fatalf("sync_alignment = %q, want true", query.Get("sync_alignment"))
			}
			if query.Get("auto_mode") != "true" {
				t.Fatalf("auto_mode = %q, want true", query.Get("auto_mode"))
			}
			if query.Get("apply_text_normalization") != "off" {
				t.Fatalf("apply_text_normalization = %q, want off", query.Get("apply_text_normalization"))
			}
			if query.Get("seed") != "123" {
				t.Fatalf("seed = %q, want 123", query.Get("seed"))
			}
			if got := r.Header.Get("xi-api-key"); got != "test-key" {
				t.Fatalf("xi-api-key = %q, want test-key", got)
			}
			return nil
		},
		Handler: func(ws *websocket.Conn) {
			var init TTSStreamInitializeMessage
			if err := websocket.JSON.Receive(ws, &init); err != nil {
				t.Errorf("receive init: %v", err)
				return
			}
			if init.Text != " " {
				t.Errorf("init text = %q, want blank space", init.Text)
			}
			if init.VoiceSettings == nil || init.VoiceSettings.Stability == nil || *init.VoiceSettings.Stability != 0.5 {
				t.Errorf("init voice settings = %#v, want stability 0.5", init.VoiceSettings)
			}
			if init.GenerationConfig == nil || len(init.GenerationConfig.ChunkLengthSchedule) != 1 || init.GenerationConfig.ChunkLengthSchedule[0] != 50 {
				t.Errorf("init generation config = %#v, want [50]", init.GenerationConfig)
			}

			var text TTSStreamTextMessage
			if err := websocket.JSON.Receive(ws, &text); err != nil {
				t.Errorf("receive text: %v", err)
				return
			}
			if text.Text != "Hello " {
				t.Errorf("text = %q, want Hello", text.Text)
			}
			if text.TryTriggerGeneration == nil || !*text.TryTriggerGeneration {
				t.Errorf("try_trigger_generation = %#v, want true", text.TryTriggerGeneration)
			}
			if text.Flush == nil || !*text.Flush {
				t.Errorf("flush = %#v, want true", text.Flush)
			}

			_ = websocket.JSON.Send(ws, TTSStreamInputEvent{
				Audio: base64.StdEncoding.EncodeToString([]byte("audio")),
				Alignment: &TTSStreamAlignment{
					Chars:            []string{"H"},
					CharStartTimesMs: []int{0},
					CharDurationsMs:  []int{20},
				},
			})

			var closeMessage TTSStreamTextMessage
			if err := websocket.JSON.Receive(ws, &closeMessage); err != nil {
				t.Errorf("receive close input: %v", err)
				return
			}
			if closeMessage.Text != "" {
				t.Errorf("close text = %q, want empty", closeMessage.Text)
			}
			_ = websocket.JSON.Send(ws, TTSStreamInputEvent{IsFinal: true})
		},
	})
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	session, err := client.TTS.ConnectStreamInput(ctx, TTSStreamInputRequest{
		VoiceID:                "voice_123",
		ModelID:                "eleven_flash_v2_5",
		LanguageCode:           "en",
		EnableLogging:          boolPtr(false),
		EnableSSMLParsing:      boolPtr(true),
		OutputFormat:           "mp3_44100_128",
		InactivityTimeout:      intPtr(30),
		SyncAlignment:          boolPtr(true),
		AutoMode:               boolPtr(true),
		ApplyTextNormalization: "off",
		Seed:                   intPtr(123),
	})
	if err != nil {
		t.Fatalf("ConnectStreamInput returned error: %v", err)
	}
	defer session.Close()

	if err := session.Initialize(TTSStreamInitializeMessage{
		VoiceSettings:    &VoiceSettings{Stability: floatPtr(0.5)},
		GenerationConfig: &GenerationConfig{ChunkLengthSchedule: []float64{50}},
	}); err != nil {
		t.Fatalf("Initialize returned error: %v", err)
	}
	if err := session.SendText(TTSStreamTextMessage{
		Text:                 "Hello ",
		TryTriggerGeneration: boolPtr(true),
		Flush:                boolPtr(true),
	}); err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}

	event, err := session.Receive()
	if err != nil {
		t.Fatalf("Receive audio returned error: %v", err)
	}
	audio, err := event.AudioBytes()
	if err != nil {
		t.Fatalf("AudioBytes returned error: %v", err)
	}
	if string(audio) != "audio" {
		t.Fatalf("audio = %q, want audio", string(audio))
	}
	if event.Alignment == nil || event.Alignment.Chars[0] != "H" {
		t.Fatalf("alignment = %#v, want H", event.Alignment)
	}

	if err := session.CloseInput(); err != nil {
		t.Fatalf("CloseInput returned error: %v", err)
	}
	final, err := session.Receive()
	if err != nil {
		t.Fatalf("Receive final returned error: %v", err)
	}
	if !final.IsFinal {
		t.Fatalf("IsFinal = false, want true")
	}
}

func TestMultiStreamInputSessionSendsContextMessages(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(websocket.Server{
		Handshake: func(_ *websocket.Config, r *http.Request) error {
			if r.URL.Path != "/v1/text-to-speech/voice_123/multi-stream-input" {
				t.Fatalf("path = %s, want multi-stream-input endpoint", r.URL.Path)
			}
			if r.URL.Query().Get("single_use_token") != "token_123" {
				t.Fatalf("single_use_token = %q, want token_123", r.URL.Query().Get("single_use_token"))
			}
			if got := r.Header.Get("xi-api-key"); got != "" {
				t.Fatalf("xi-api-key = %q, want empty with token auth", got)
			}
			return nil
		},
		Handler: func(ws *websocket.Conn) {
			var init TTSMultiStreamContextMessage
			if err := websocket.JSON.Receive(ws, &init); err != nil {
				t.Errorf("receive init: %v", err)
				return
			}
			if init.Text != " " || init.ContextID != "ctx_a" {
				t.Errorf("init = %+v, want blank text for ctx_a", init)
			}

			var text TTSMultiStreamTextMessage
			if err := websocket.JSON.Receive(ws, &text); err != nil {
				t.Errorf("receive text: %v", err)
				return
			}
			if text.Text != "Hello " || text.ContextID != "ctx_a" {
				t.Errorf("text = %+v, want Hello for ctx_a", text)
			}

			_ = websocket.JSON.Send(ws, TTSStreamInputEvent{
				ContextID: "ctx_a",
				Audio:     base64.StdEncoding.EncodeToString([]byte("ctx-audio")),
			})

			var flush TTSMultiStreamFlushMessage
			if err := websocket.JSON.Receive(ws, &flush); err != nil {
				t.Errorf("receive flush: %v", err)
				return
			}
			if flush.ContextID != "ctx_a" || flush.Text != "tail " || !flush.Flush {
				t.Errorf("flush = %+v, want ctx_a tail flush", flush)
			}

			var closeContext map[string]any
			if err := websocket.JSON.Receive(ws, &closeContext); err != nil {
				t.Errorf("receive close context: %v", err)
				return
			}
			if closeContext["context_id"] != "ctx_a" || closeContext["close_context"] != true {
				t.Errorf("close context = %#v, want ctx_a close", closeContext)
			}

			var closeSocket map[string]any
			if err := websocket.JSON.Receive(ws, &closeSocket); err != nil {
				t.Errorf("receive close socket: %v", err)
				return
			}
			if closeSocket["close_socket"] != true {
				t.Errorf("close socket = %#v, want close_socket true", closeSocket)
			}
		},
	})
	defer server.Close()

	client := NewClient("", WithBaseURL(server.URL), WithHTTPClient(server.Client()))

	session, err := client.TTS.ConnectMultiStreamInput(ctx, TTSStreamInputRequest{
		VoiceID:        "voice_123",
		SingleUseToken: "token_123",
	})
	if err != nil {
		t.Fatalf("ConnectMultiStreamInput returned error: %v", err)
	}
	defer session.Close()

	if err := session.Initialize(TTSMultiStreamContextMessage{ContextID: "ctx_a"}); err != nil {
		t.Fatalf("Initialize returned error: %v", err)
	}
	if err := session.SendText(TTSMultiStreamTextMessage{Text: "Hello ", ContextID: "ctx_a"}); err != nil {
		t.Fatalf("SendText returned error: %v", err)
	}
	event, err := session.Receive()
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}
	if event.ContextID != "ctx_a" {
		t.Fatalf("ContextID = %q, want ctx_a", event.ContextID)
	}
	audio, err := event.AudioBytes()
	if err != nil {
		t.Fatalf("AudioBytes returned error: %v", err)
	}
	if string(audio) != "ctx-audio" {
		t.Fatalf("audio = %q, want ctx-audio", string(audio))
	}
	if err := session.FlushContext(TTSMultiStreamFlushMessage{ContextID: "ctx_a", Text: "tail "}); err != nil {
		t.Fatalf("FlushContext returned error: %v", err)
	}
	if err := session.CloseContext("ctx_a"); err != nil {
		t.Fatalf("CloseContext returned error: %v", err)
	}
	if err := session.CloseSocket(); err != nil {
		t.Fatalf("CloseSocket returned error: %v", err)
	}
}
