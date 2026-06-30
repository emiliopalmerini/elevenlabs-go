package elevenlabs

import (
	"encoding/base64"
	"errors"
	"io"
)

// CreateSpeechRequest contains parameters for ElevenLabs text-to-speech HTTP
// requests.
type CreateSpeechRequest struct {
	VoiceID string `json:"-"`
	Text    string `json:"text"`

	ModelID       string         `json:"model_id,omitempty"`
	LanguageCode  string         `json:"language_code,omitempty"`
	VoiceSettings *VoiceSettings `json:"voice_settings,omitempty"`

	PronunciationDictionaryLocators []PronunciationDictionaryLocator `json:"pronunciation_dictionary_locators,omitempty"`
	Seed                            *int                             `json:"seed,omitempty"`
	PreviousText                    string                           `json:"previous_text,omitempty"`
	NextText                        string                           `json:"next_text,omitempty"`
	PreviousRequestIDs              []string                         `json:"previous_request_ids,omitempty"`
	NextRequestIDs                  []string                         `json:"next_request_ids,omitempty"`
	UsePVCAsIVC                     *bool                            `json:"use_pvc_as_ivc,omitempty"`
	ApplyTextNormalization          string                           `json:"apply_text_normalization,omitempty"`
	ApplyLanguageTextNormalization  *bool                            `json:"apply_language_text_normalization,omitempty"`

	EnableLogging            *bool        `json:"-"`
	OptimizeStreamingLatency *int         `json:"-"`
	OutputFormat             OutputFormat `json:"-"`
}

// VoiceSettings overrides stored voice settings for one request.
type VoiceSettings struct {
	Stability       *float64 `json:"stability,omitempty"`
	SimilarityBoost *float64 `json:"similarity_boost,omitempty"`
	Style           *float64 `json:"style,omitempty"`
	UseSpeakerBoost *bool    `json:"use_speaker_boost,omitempty"`
	Speed           *float64 `json:"speed,omitempty"`
}

// PronunciationDictionaryLocator identifies a pronunciation dictionary version.
type PronunciationDictionaryLocator struct {
	PronunciationDictionaryID string `json:"pronunciation_dictionary_id"`
	VersionID                 string `json:"version_id,omitempty"`
}

// GenerationConfig configures WebSocket text buffering before audio generation.
type GenerationConfig struct {
	ChunkLengthSchedule []float64 `json:"chunk_length_schedule,omitempty"`
}

// AudioWithTimestamps is returned by timestamp-enabled text-to-speech APIs.
type AudioWithTimestamps struct {
	AudioBase64         string              `json:"audio_base64"`
	Alignment           *CharacterAlignment `json:"alignment,omitempty"`
	NormalizedAlignment *CharacterAlignment `json:"normalized_alignment,omitempty"`
}

// Audio decodes the base64-encoded audio in AudioBase64.
func (a AudioWithTimestamps) Audio() ([]byte, error) {
	return base64.StdEncoding.DecodeString(a.AudioBase64)
}

// CharacterAlignment contains character-level timing information in seconds.
type CharacterAlignment struct {
	Characters                 []string  `json:"characters"`
	CharacterStartTimesSeconds []float64 `json:"character_start_times_seconds"`
	CharacterEndTimesSeconds   []float64 `json:"character_end_times_seconds"`
}

// AudioStream is a streaming HTTP audio response. The caller must close it.
type AudioStream struct {
	Body io.ReadCloser
}

func newAudioStream(body io.ReadCloser) *AudioStream {
	return &AudioStream{Body: body}
}

// Read reads audio bytes from the response stream.
func (s *AudioStream) Read(p []byte) (int, error) {
	if s == nil || s.Body == nil {
		return 0, errors.New("elevenlabs: nil audio stream")
	}
	return s.Body.Read(p)
}

// Close closes the response stream.
func (s *AudioStream) Close() error {
	if s == nil || s.Body == nil {
		return nil
	}
	return s.Body.Close()
}

// TTSStreamAlignment contains WebSocket chunk alignment information in
// milliseconds.
type TTSStreamAlignment struct {
	CharStartTimesMs []int    `json:"charStartTimesMs,omitempty"`
	CharDurationsMs  []int    `json:"charDurationsMs,omitempty"`
	Chars            []string `json:"chars,omitempty"`
}

// TTSStreamInputEvent is a JSON event received from single or multi-context
// text-to-speech WebSocket streams.
type TTSStreamInputEvent struct {
	Audio               string              `json:"audio,omitempty"`
	Alignment           *TTSStreamAlignment `json:"alignment,omitempty"`
	NormalizedAlignment *TTSStreamAlignment `json:"normalizedAlignment,omitempty"`
	IsFinal             bool                `json:"isFinal,omitempty"`
	ContextID           string              `json:"contextId,omitempty"`
	Error               string              `json:"error,omitempty"`
}

// AudioBytes decodes a base64-encoded WebSocket audio chunk.
func (e TTSStreamInputEvent) AudioBytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString(e.Audio)
}
