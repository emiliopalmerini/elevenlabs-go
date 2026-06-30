package elevenlabs

// SpeechEngineConfig configures the upstream WebSocket endpoint that
// ElevenLabs connects to for Speech Engine conversations.
type SpeechEngineConfig struct {
	RequestHeaders map[string]any `json:"request_headers,omitempty"`
	WSURL          string         `json:"ws_url"`
}

// SpeechEngineASRConfig configures automatic speech recognition for a Speech
// Engine resource.
type SpeechEngineASRConfig struct {
	Keywords             []string `json:"keywords,omitempty"`
	Provider             string   `json:"provider,omitempty"`
	Quality              string   `json:"quality,omitempty"`
	UserInputAudioFormat string   `json:"user_input_audio_format,omitempty"`
}

// SpeechEngineTTSConfig configures text-to-speech output for a Speech Engine
// resource.
type SpeechEngineTTSConfig struct {
	AgentOutputAudioFormat          string                           `json:"agent_output_audio_format,omitempty"`
	EnablePhonemeTags               *bool                            `json:"enable_phoneme_tags,omitempty"`
	ExpressiveMode                  *bool                            `json:"expressive_mode,omitempty"`
	ModelID                         string                           `json:"model_id,omitempty"`
	OptimizeStreamingLatency        *int                             `json:"optimize_streaming_latency,omitempty"`
	PronunciationDictionaryLocators []PronunciationDictionaryLocator `json:"pronunciation_dictionary_locators,omitempty"`
	SimilarityBoost                 *float64                         `json:"similarity_boost,omitempty"`
	Speed                           *float64                         `json:"speed,omitempty"`
	Stability                       *float64                         `json:"stability,omitempty"`
	SuggestedAudioTags              []string                         `json:"suggested_audio_tags,omitempty"`
	SupportedVoices                 []map[string]any                 `json:"supported_voices,omitempty"`
	TextNormalisationType           string                           `json:"text_normalisation_type,omitempty"`
	VoiceID                         string                           `json:"voice_id,omitempty"`
}

// SpeechEngineTurnConfig configures turn detection and interruption behavior.
type SpeechEngineTurnConfig struct {
	InitialWaitTime                   *float64 `json:"initial_wait_time,omitempty"`
	InterruptionIgnoreTerms           []string `json:"interruption_ignore_terms,omitempty"`
	RetranscribeOnTurnTimeout         *bool    `json:"retranscribe_on_turn_timeout,omitempty"`
	SilenceEndCallTimeout             *float64 `json:"silence_end_call_timeout,omitempty"`
	SpeculativeTurn                   *bool    `json:"speculative_turn,omitempty"`
	SpellingPatience                  string   `json:"spelling_patience,omitempty"`
	TranscribeOnDisabledInterruptions *bool    `json:"transcribe_on_disabled_interruptions,omitempty"`
	TurnEagerness                     string   `json:"turn_eagerness,omitempty"`
	TurnModel                         string   `json:"turn_model,omitempty"`
	TurnTimeout                       *float64 `json:"turn_timeout,omitempty"`
}

// SpeechEnginePrivacyConfig configures recording, retention, and PII handling
// for Speech Engine conversations.
type SpeechEnginePrivacyConfig struct {
	ApplyToExistingConversations *bool          `json:"apply_to_existing_conversations,omitempty"`
	ConversationHistoryRedaction map[string]any `json:"conversation_history_redaction,omitempty"`
	DeleteAudio                  *bool          `json:"delete_audio,omitempty"`
	DeleteTranscriptAndPII       *bool          `json:"delete_transcript_and_pii,omitempty"`
	RecordVoice                  *bool          `json:"record_voice,omitempty"`
	RetentionDays                *int           `json:"retention_days,omitempty"`
	ZeroRetentionMode            *bool          `json:"zero_retention_mode,omitempty"`
}

// SpeechEngineCallLimits configures per-engine concurrency and daily usage
// limits.
type SpeechEngineCallLimits struct {
	AgentConcurrencyLimit *int  `json:"agent_concurrency_limit,omitempty"`
	BurstingEnabled       *bool `json:"bursting_enabled,omitempty"`
	DailyLimit            *int  `json:"daily_limit,omitempty"`
}

// SpeechEngineCreateRequest contains parameters for creating a Speech Engine
// resource.
type SpeechEngineCreateRequest struct {
	ASR          *SpeechEngineASRConfig     `json:"asr,omitempty"`
	CallLimits   *SpeechEngineCallLimits    `json:"call_limits,omitempty"`
	Conversation map[string]any             `json:"conversation,omitempty"`
	Language     string                     `json:"language,omitempty"`
	Name         string                     `json:"name,omitempty"`
	Overrides    map[string]any             `json:"overrides,omitempty"`
	Privacy      *SpeechEnginePrivacyConfig `json:"privacy,omitempty"`
	SpeechEngine SpeechEngineConfig         `json:"speech_engine"`
	Tags         []string                   `json:"tags,omitempty"`
	TTS          *SpeechEngineTTSConfig     `json:"tts,omitempty"`
	Turn         *SpeechEngineTurnConfig    `json:"turn,omitempty"`
}

// SpeechEngineUpdateRequest contains parameters for partially updating a
// Speech Engine resource.
type SpeechEngineUpdateRequest struct {
	ASR          *SpeechEngineASRConfig     `json:"asr,omitempty"`
	CallLimits   *SpeechEngineCallLimits    `json:"call_limits,omitempty"`
	Conversation map[string]any             `json:"conversation,omitempty"`
	Language     string                     `json:"language,omitempty"`
	Name         string                     `json:"name,omitempty"`
	Privacy      *SpeechEnginePrivacyConfig `json:"privacy,omitempty"`
	SpeechEngine *SpeechEngineConfig        `json:"speech_engine,omitempty"`
	Tags         []string                   `json:"tags,omitempty"`
	TTS          *SpeechEngineTTSConfig     `json:"tts,omitempty"`
	Turn         *SpeechEngineTurnConfig    `json:"turn,omitempty"`
}

// SpeechEngineResponse is the full Speech Engine resource returned by create,
// get, and update APIs.
type SpeechEngineResponse struct {
	AccessInfo     map[string]any             `json:"access_info,omitempty"`
	ASR            *SpeechEngineASRConfig     `json:"asr,omitempty"`
	CallLimits     *SpeechEngineCallLimits    `json:"call_limits,omitempty"`
	Conversation   map[string]any             `json:"conversation,omitempty"`
	Language       string                     `json:"language,omitempty"`
	Metadata       map[string]any             `json:"metadata,omitempty"`
	Name           string                     `json:"name"`
	Overrides      map[string]any             `json:"overrides,omitempty"`
	Privacy        *SpeechEnginePrivacyConfig `json:"privacy,omitempty"`
	SpeechEngine   SpeechEngineConfig         `json:"speech_engine"`
	SpeechEngineID string                     `json:"speech_engine_id"`
	Tags           []string                   `json:"tags,omitempty"`
	TTS            *SpeechEngineTTSConfig     `json:"tts,omitempty"`
	Turn           *SpeechEngineTurnConfig    `json:"turn,omitempty"`
}

// SpeechEngineSummaryResponse is one item in a paginated Speech Engine list
// response.
type SpeechEngineSummaryResponse struct {
	AccessInfo        map[string]any `json:"access_info,omitempty"`
	CreatedAtUnixSecs int64          `json:"created_at_unix_secs"`
	Name              string         `json:"name"`
	SpeechEngineID    string         `json:"speech_engine_id"`
	Tags              []string       `json:"tags,omitempty"`
}

// ListSpeechEnginesResponse contains one page of Speech Engine resources.
type ListSpeechEnginesResponse struct {
	HasMore       bool                          `json:"has_more"`
	NextCursor    *string                       `json:"next_cursor,omitempty"`
	SpeechEngines []SpeechEngineSummaryResponse `json:"speech_engines"`
}
