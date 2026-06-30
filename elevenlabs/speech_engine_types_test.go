package elevenlabs

import (
	"encoding/json"
	"testing"
)

func TestSpeechEngineTypesRoundTripDocumentedFields(t *testing.T) {
	body := []byte(`{
		"speech_engine_id": "seng_123",
		"name": "Orders",
		"speech_engine": {
			"ws_url": "wss://example.com/speech-engine",
			"request_headers": {
				"x-tenant": "acme"
			}
		},
		"asr": {
			"quality": "high",
			"provider": "elevenlabs",
			"user_input_audio_format": "pcm_16000",
			"keywords": ["pizza", "delivery"]
		},
		"tts": {
			"model_id": "eleven_flash_v2_5",
			"voice_id": "voice_123",
			"agent_output_audio_format": "mp3_44100_128",
			"optimize_streaming_latency": 2,
			"stability": 0.5,
			"speed": 1.1,
			"similarity_boost": 0.8,
			"expressive_mode": true,
			"enable_phoneme_tags": true
		},
		"turn": {
			"turn_timeout": 7.5,
			"initial_wait_time": 2,
			"silence_end_call_timeout": 30,
			"turn_eagerness": "high",
			"spelling_patience": "auto",
			"speculative_turn": true,
			"retranscribe_on_turn_timeout": true,
			"interruption_ignore_terms": ["gotcha"],
			"transcribe_on_disabled_interruptions": true
		},
		"privacy": {
			"record_voice": false,
			"retention_days": 7,
			"delete_transcript_and_pii": true,
			"delete_audio": true,
			"apply_to_existing_conversations": true,
			"zero_retention_mode": true
		},
		"call_limits": {
			"agent_concurrency_limit": 5,
			"daily_limit": 100,
			"bursting_enabled": true
		},
		"language": "en",
		"tags": ["prod", "support"],
		"overrides": {"first_message": true},
		"metadata": {"created_at_unix_secs": 1700000000},
		"access_info": {"is_creator": true}
	}`)

	var engine SpeechEngineResponse
	if err := json.Unmarshal(body, &engine); err != nil {
		t.Fatalf("unmarshal SpeechEngineResponse: %v", err)
	}

	if engine.SpeechEngineID != "seng_123" {
		t.Fatalf("SpeechEngineID = %q, want seng_123", engine.SpeechEngineID)
	}
	if engine.SpeechEngine.WSURL != "wss://example.com/speech-engine" {
		t.Fatalf("WSURL = %q, want upstream URL", engine.SpeechEngine.WSURL)
	}
	if engine.SpeechEngine.RequestHeaders["x-tenant"] != "acme" {
		t.Fatalf("RequestHeaders = %#v, want x-tenant", engine.SpeechEngine.RequestHeaders)
	}
	if engine.ASR == nil || engine.ASR.Quality != "high" || len(engine.ASR.Keywords) != 2 {
		t.Fatalf("ASR = %#v, want documented ASR fields", engine.ASR)
	}
	if engine.TTS == nil || engine.TTS.ModelID != "eleven_flash_v2_5" || engine.TTS.VoiceID != "voice_123" {
		t.Fatalf("TTS = %#v, want model and voice", engine.TTS)
	}
	if engine.TTS.OptimizeStreamingLatency == nil || *engine.TTS.OptimizeStreamingLatency != 2 {
		t.Fatalf("OptimizeStreamingLatency = %#v, want 2", engine.TTS.OptimizeStreamingLatency)
	}
	if engine.TTS.Stability == nil || *engine.TTS.Stability != 0.5 {
		t.Fatalf("Stability = %#v, want 0.5", engine.TTS.Stability)
	}
	if engine.Turn == nil || engine.Turn.TurnTimeout == nil || *engine.Turn.TurnTimeout != 7.5 {
		t.Fatalf("Turn = %#v, want timeout 7.5", engine.Turn)
	}
	if engine.Privacy == nil || engine.Privacy.RecordVoice == nil || *engine.Privacy.RecordVoice {
		t.Fatalf("Privacy.RecordVoice = %#v, want false", engine.Privacy)
	}
	if engine.CallLimits == nil || engine.CallLimits.DailyLimit == nil || *engine.CallLimits.DailyLimit != 100 {
		t.Fatalf("CallLimits = %#v, want daily limit 100", engine.CallLimits)
	}
	if engine.Language != "en" || len(engine.Tags) != 2 || engine.Tags[1] != "support" {
		t.Fatalf("language/tags = %q %#v, want en support tags", engine.Language, engine.Tags)
	}
	if engine.Overrides["first_message"] != true {
		t.Fatalf("Overrides = %#v, want first_message", engine.Overrides)
	}

	encoded, err := json.Marshal(SpeechEngineCreateRequest{
		Name: "Orders",
		SpeechEngine: SpeechEngineConfig{
			WSURL: "wss://example.com/speech-engine",
		},
		ASR: &SpeechEngineASRConfig{
			Provider: "elevenlabs",
		},
	})
	if err != nil {
		t.Fatalf("marshal SpeechEngineCreateRequest: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	if payload["name"] != "Orders" {
		t.Fatalf("name = %#v, want Orders", payload["name"])
	}
	speechEngine, ok := payload["speech_engine"].(map[string]any)
	if !ok || speechEngine["ws_url"] != "wss://example.com/speech-engine" {
		t.Fatalf("speech_engine = %#v, want ws_url", payload["speech_engine"])
	}
	asr, ok := payload["asr"].(map[string]any)
	if !ok || asr["provider"] != "elevenlabs" {
		t.Fatalf("asr = %#v, want provider", payload["asr"])
	}
	if _, ok := payload["tts"]; ok {
		t.Fatalf("tts was encoded despite being empty: %#v", payload["tts"])
	}
}

func TestListSpeechEnginesResponseParsesPagination(t *testing.T) {
	var out ListSpeechEnginesResponse
	if err := json.Unmarshal([]byte(`{
		"speech_engines": [
			{
				"speech_engine_id": "seng_123",
				"name": "Orders",
				"created_at_unix_secs": 1700000000,
				"tags": ["prod"],
				"access_info": {"is_creator": true}
			}
		],
		"next_cursor": "cursor_2",
		"has_more": true
	}`), &out); err != nil {
		t.Fatalf("unmarshal ListSpeechEnginesResponse: %v", err)
	}

	if len(out.SpeechEngines) != 1 {
		t.Fatalf("SpeechEngines length = %d, want 1", len(out.SpeechEngines))
	}
	engine := out.SpeechEngines[0]
	if engine.SpeechEngineID != "seng_123" || engine.Name != "Orders" {
		t.Fatalf("summary = %#v, want seng_123 Orders", engine)
	}
	if engine.CreatedAtUnixSecs != 1700000000 {
		t.Fatalf("CreatedAtUnixSecs = %d, want 1700000000", engine.CreatedAtUnixSecs)
	}
	if out.NextCursor == nil || *out.NextCursor != "cursor_2" || !out.HasMore {
		t.Fatalf("pagination = cursor %#v has_more %v, want cursor_2 true", out.NextCursor, out.HasMore)
	}
}
