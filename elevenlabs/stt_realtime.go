package elevenlabs

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/websocket"
)

// RealtimeTranscriptRequest configures a realtime speech-to-text session.
type RealtimeTranscriptRequest struct {
	ModelID                  string
	Token                    string
	IncludeTimestamps        *bool
	IncludeLanguageDetection *bool
	AudioFormat              string
	LanguageCode             string
	CommitStrategy           string
	Keyterms                 []string
	NoVerbatim               *bool
	VADSilenceThresholdSecs  *float64
	VADThreshold             *float64
	MinSpeechDurationMS      *int
	MinSilenceDurationMS     *int
	EnableLogging            *bool
}

// RealtimeTranscriptSession is an active realtime speech-to-text WebSocket
// session.
type RealtimeTranscriptSession struct {
	conn *websocket.Conn
}

// RealtimeAudioChunk is an audio chunk sent to a realtime transcription session.
type RealtimeAudioChunk struct {
	Audio        []byte
	AudioBase64  string
	Commit       bool
	SampleRate   int
	PreviousText string
}

type realtimeAudioChunkMessage struct {
	MessageType  string `json:"message_type"`
	AudioBase64  string `json:"audio_base_64"`
	Commit       bool   `json:"commit"`
	SampleRate   int    `json:"sample_rate"`
	PreviousText string `json:"previous_text,omitempty"`
}

// RealtimeTranscriptEvent is a JSON event received from a realtime
// speech-to-text session.
type RealtimeTranscriptEvent struct {
	MessageType  string                   `json:"message_type"`
	SessionID    string                   `json:"session_id,omitempty"`
	Config       RealtimeTranscriptConfig `json:"config,omitempty"`
	Text         string                   `json:"text,omitempty"`
	LanguageCode string                   `json:"language_code,omitempty"`
	Words        []RealtimeTranscriptWord `json:"words,omitempty"`
	Error        string                   `json:"error,omitempty"`
}

// RealtimeTranscriptConfig describes the started realtime transcription session.
type RealtimeTranscriptConfig struct {
	SampleRate               int      `json:"sample_rate,omitempty"`
	AudioFormat              string   `json:"audio_format,omitempty"`
	LanguageCode             string   `json:"language_code,omitempty"`
	CommitStrategy           string   `json:"commit_strategy,omitempty"`
	VADSilenceThresholdSecs  float64  `json:"vad_silence_threshold_secs,omitempty"`
	VADThreshold             float64  `json:"vad_threshold,omitempty"`
	MinSpeechDurationMS      int      `json:"min_speech_duration_ms,omitempty"`
	MinSilenceDurationMS     int      `json:"min_silence_duration_ms,omitempty"`
	ModelID                  string   `json:"model_id,omitempty"`
	EnableLogging            bool     `json:"enable_logging,omitempty"`
	IncludeTimestamps        bool     `json:"include_timestamps,omitempty"`
	IncludeLanguageDetection bool     `json:"include_language_detection,omitempty"`
	Keyterms                 []string `json:"keyterms,omitempty"`
	NoVerbatim               bool     `json:"no_verbatim,omitempty"`
}

// RealtimeTranscriptWord is word-level data in a realtime transcript event.
type RealtimeTranscriptWord struct {
	Text       string   `json:"text"`
	Start      float64  `json:"start,omitempty"`
	End        float64  `json:"end,omitempty"`
	Type       string   `json:"type,omitempty"`
	SpeakerID  string   `json:"speaker_id,omitempty"`
	Logprob    float64  `json:"logprob,omitempty"`
	Characters []string `json:"characters,omitempty"`
}

// ConnectRealtimeTranscript opens a realtime speech-to-text WebSocket session.
func (c *STTService) ConnectRealtimeTranscript(ctx context.Context, in RealtimeTranscriptRequest) (*RealtimeTranscriptSession, error) {
	core, err := c.apiClient()
	if err != nil {
		return nil, err
	}
	endpoint, origin, err := c.realtimeTranscriptEndpoint(core, in)
	if err != nil {
		return nil, err
	}
	config, err := websocket.NewConfig(endpoint, origin)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: create realtime websocket config: %w", err)
	}
	if strings.TrimSpace(in.Token) == "" {
		header, err := core.AuthHeader()
		if err != nil {
			if strings.Contains(err.Error(), "api key is required") {
				return nil, errors.New("elevenlabs: api key or realtime token is required")
			}
			return nil, err
		}
		config.Header = header
	}

	conn, err := config.DialContext(ctx)
	if err != nil {
		return nil, err
	}
	return &RealtimeTranscriptSession{conn: conn}, nil
}

func (c *STTService) realtimeTranscriptEndpoint(core *Client, in RealtimeTranscriptRequest) (string, string, error) {
	baseURL, err := core.Endpoint("/v1/speech-to-text/realtime")
	if err != nil {
		return "", "", err
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", "", fmt.Errorf("elevenlabs: parse realtime endpoint: %w", err)
	}

	origin := *base
	switch origin.Scheme {
	case "ws":
		origin.Scheme = "http"
	case "wss":
		origin.Scheme = "https"
	}
	origin.Path = ""
	origin.RawQuery = ""
	origin.Fragment = ""

	wsBase := *base
	switch wsBase.Scheme {
	case "http":
		wsBase.Scheme = "ws"
	case "https":
		wsBase.Scheme = "wss"
	}

	endpoint := &wsBase
	query := endpoint.Query()
	setStringQuery(query, "model_id", in.ModelID)
	setStringQuery(query, "token", in.Token)
	setBoolQuery(query, "include_timestamps", in.IncludeTimestamps)
	setBoolQuery(query, "include_language_detection", in.IncludeLanguageDetection)
	setStringQuery(query, "audio_format", in.AudioFormat)
	setStringQuery(query, "language_code", in.LanguageCode)
	setStringQuery(query, "commit_strategy", in.CommitStrategy)
	for _, keyterm := range in.Keyterms {
		query.Add("keyterms", keyterm)
	}
	setBoolQuery(query, "no_verbatim", in.NoVerbatim)
	setFloatQuery(query, "vad_silence_threshold_secs", in.VADSilenceThresholdSecs)
	setFloatQuery(query, "vad_threshold", in.VADThreshold)
	setIntQuery(query, "min_speech_duration_ms", in.MinSpeechDurationMS)
	setIntQuery(query, "min_silence_duration_ms", in.MinSilenceDurationMS)
	setBoolQuery(query, "enable_logging", in.EnableLogging)
	endpoint.RawQuery = query.Encode()

	return endpoint.String(), origin.String(), nil
}

func setStringQuery(query url.Values, name, value string) {
	if strings.TrimSpace(value) != "" {
		query.Set(name, value)
	}
}

func setBoolQuery(query url.Values, name string, value *bool) {
	if value != nil {
		query.Set(name, strconv.FormatBool(*value))
	}
}

func setFloatQuery(query url.Values, name string, value *float64) {
	if value != nil {
		query.Set(name, strconv.FormatFloat(*value, 'f', -1, 64))
	}
}

func setIntQuery(query url.Values, name string, value *int) {
	if value != nil {
		query.Set(name, strconv.Itoa(*value))
	}
}

// SendAudioChunk sends one audio chunk to the realtime transcription session.
func (s *RealtimeTranscriptSession) SendAudioChunk(chunk RealtimeAudioChunk) error {
	if s == nil || s.conn == nil {
		return errors.New("elevenlabs: nil realtime transcript session")
	}
	audioBase64 := chunk.AudioBase64
	if audioBase64 == "" && len(chunk.Audio) > 0 {
		audioBase64 = base64.StdEncoding.EncodeToString(chunk.Audio)
	}
	message := realtimeAudioChunkMessage{
		MessageType:  "input_audio_chunk",
		AudioBase64:  audioBase64,
		Commit:       chunk.Commit,
		SampleRate:   chunk.SampleRate,
		PreviousText: chunk.PreviousText,
	}
	return websocket.JSON.Send(s.conn, message)
}

// Receive reads one event from the realtime transcription session.
func (s *RealtimeTranscriptSession) Receive() (*RealtimeTranscriptEvent, error) {
	if s == nil || s.conn == nil {
		return nil, errors.New("elevenlabs: nil realtime transcript session")
	}
	var event RealtimeTranscriptEvent
	if err := websocket.JSON.Receive(s.conn, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// Close closes the realtime transcription session.
func (s *RealtimeTranscriptSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}
