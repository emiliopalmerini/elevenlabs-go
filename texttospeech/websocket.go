package texttospeech

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	elevenlabs "github.com/emiliopalmerini/elevenlabs-go"
	"golang.org/x/net/websocket"
)

// StreamInputRequest configures a text-to-speech WebSocket connection.
type StreamInputRequest struct {
	VoiceID string

	Authorization  string
	SingleUseToken string
	ModelID        string
	LanguageCode   string

	EnableLogging          *bool
	EnableSSMLParsing      *bool
	OutputFormat           string
	InactivityTimeout      *int
	SyncAlignment          *bool
	AutoMode               *bool
	ApplyTextNormalization string
	Seed                   *int
}

// StreamInputSession is an active text-to-speech WebSocket session.
type StreamInputSession struct {
	conn *websocket.Conn
}

// MultiStreamInputSession is an active multi-context text-to-speech WebSocket
// session.
type MultiStreamInputSession struct {
	conn *websocket.Conn
}

// StreamInitializeMessage is the first message sent to a single-context
// WebSocket stream.
type StreamInitializeMessage struct {
	Text                            string                           `json:"text"`
	VoiceSettings                   *VoiceSettings                   `json:"voice_settings,omitempty"`
	GenerationConfig                *GenerationConfig                `json:"generation_config,omitempty"`
	PronunciationDictionaryLocators []PronunciationDictionaryLocator `json:"pronunciation_dictionary_locators,omitempty"`
	APIKey                          string                           `json:"xi-api-key,omitempty"`
	Authorization                   string                           `json:"authorization,omitempty"`
}

// StreamTextMessage sends text to a single-context WebSocket stream.
type StreamTextMessage struct {
	Text                 string            `json:"text"`
	TryTriggerGeneration *bool             `json:"try_trigger_generation,omitempty"`
	VoiceSettings        *VoiceSettings    `json:"voice_settings,omitempty"`
	GeneratorConfig      *GenerationConfig `json:"generator_config,omitempty"`
	Flush                *bool             `json:"flush,omitempty"`
}

// MultiStreamContextMessage initializes or re-initializes one multi-context
// WebSocket context.
type MultiStreamContextMessage struct {
	Text                            string                           `json:"text"`
	VoiceSettings                   *VoiceSettings                   `json:"voice_settings,omitempty"`
	GenerationConfig                *GenerationConfig                `json:"generation_config,omitempty"`
	PronunciationDictionaryLocators []PronunciationDictionaryLocator `json:"pronunciation_dictionary_locators,omitempty"`
	APIKey                          string                           `json:"xi_api_key,omitempty"`
	Authorization                   string                           `json:"authorization,omitempty"`
	ContextID                       string                           `json:"context_id,omitempty"`
}

// MultiStreamTextMessage sends text to a multi-context WebSocket context.
type MultiStreamTextMessage struct {
	Text      string `json:"text"`
	ContextID string `json:"context_id,omitempty"`
	Flush     *bool  `json:"flush,omitempty"`
}

// MultiStreamFlushMessage flushes one multi-context WebSocket context.
type MultiStreamFlushMessage struct {
	ContextID string `json:"context_id"`
	Text      string `json:"text,omitempty"`
	Flush     bool   `json:"flush"`
}

// ConnectStreamInput opens a text-to-speech WebSocket session.
func (c *Client) ConnectStreamInput(ctx context.Context, in StreamInputRequest) (*StreamInputSession, error) {
	conn, err := c.connectInput(ctx, in, "/stream-input")
	if err != nil {
		return nil, err
	}
	return &StreamInputSession{conn: conn}, nil
}

// ConnectMultiStreamInput opens a multi-context text-to-speech WebSocket
// session.
func (c *Client) ConnectMultiStreamInput(ctx context.Context, in StreamInputRequest) (*MultiStreamInputSession, error) {
	conn, err := c.connectInput(ctx, in, "/multi-stream-input")
	if err != nil {
		return nil, err
	}
	return &MultiStreamInputSession{conn: conn}, nil
}

func (c *Client) connectInput(ctx context.Context, in StreamInputRequest, suffix string) (*websocket.Conn, error) {
	core, err := c.apiClient()
	if err != nil {
		return nil, err
	}
	endpoint, origin, useAPIKeyHeader, err := streamInputEndpoint(core, in, suffix)
	if err != nil {
		return nil, err
	}
	config, err := websocket.NewConfig(endpoint, origin)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: create text-to-speech websocket config: %w", err)
	}
	if useAPIKeyHeader {
		header, err := core.AuthHeader()
		if err != nil {
			if strings.Contains(err.Error(), "api key is required") {
				return nil, errors.New("elevenlabs: api key, authorization, or single use token is required")
			}
			return nil, err
		}
		config.Header = header
	}

	return config.DialContext(ctx)
}

func streamInputEndpoint(core *elevenlabs.Client, in StreamInputRequest, suffix string) (string, string, bool, error) {
	if strings.TrimSpace(in.VoiceID) == "" {
		return "", "", false, errors.New("elevenlabs: voice_id is required")
	}

	baseURL, err := core.Endpoint("/v1/text-to-speech/" + url.PathEscape(strings.TrimSpace(in.VoiceID)) + suffix)
	if err != nil {
		return "", "", false, err
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", "", false, fmt.Errorf("elevenlabs: parse text-to-speech websocket endpoint: %w", err)
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
	setStringQuery(query, "authorization", in.Authorization)
	setStringQuery(query, "single_use_token", in.SingleUseToken)
	setStringQuery(query, "model_id", in.ModelID)
	setStringQuery(query, "language_code", in.LanguageCode)
	setBoolQuery(query, "enable_logging", in.EnableLogging)
	setBoolQuery(query, "enable_ssml_parsing", in.EnableSSMLParsing)
	setStringQuery(query, "output_format", in.OutputFormat)
	setIntQuery(query, "inactivity_timeout", in.InactivityTimeout)
	setBoolQuery(query, "sync_alignment", in.SyncAlignment)
	setBoolQuery(query, "auto_mode", in.AutoMode)
	setStringQuery(query, "apply_text_normalization", in.ApplyTextNormalization)
	setIntQuery(query, "seed", in.Seed)
	endpoint.RawQuery = query.Encode()

	useAPIKeyHeader := strings.TrimSpace(in.Authorization) == "" && strings.TrimSpace(in.SingleUseToken) == ""
	return endpoint.String(), origin.String(), useAPIKeyHeader, nil
}

// Initialize sends the required first single-context WebSocket message. When
// Text is empty, it defaults to one blank space.
func (s *StreamInputSession) Initialize(in StreamInitializeMessage) error {
	if in.Text == "" {
		in.Text = " "
	}
	return s.send(in)
}

// SendText sends text to a single-context WebSocket stream.
func (s *StreamInputSession) SendText(in StreamTextMessage) error {
	return s.send(in)
}

// Flush forces audio generation for buffered single-context text.
func (s *StreamInputSession) Flush(text string) error {
	flush := true
	return s.SendText(StreamTextMessage{Text: text, Flush: &flush})
}

// CloseInput sends the empty text message that closes a single-context input
// stream.
func (s *StreamInputSession) CloseInput() error {
	return s.SendText(StreamTextMessage{Text: ""})
}

// Receive reads one event from a single-context WebSocket stream.
func (s *StreamInputSession) Receive() (*StreamInputEvent, error) {
	if s == nil || s.conn == nil {
		return nil, errors.New("elevenlabs: nil text-to-speech stream session")
	}
	var event StreamInputEvent
	if err := websocket.JSON.Receive(s.conn, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// Close closes the single-context WebSocket stream.
func (s *StreamInputSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *StreamInputSession) send(v any) error {
	if s == nil || s.conn == nil {
		return errors.New("elevenlabs: nil text-to-speech stream session")
	}
	return websocket.JSON.Send(s.conn, v)
}

// Initialize sends the required first multi-context WebSocket message. When
// Text is empty, it defaults to one blank space.
func (s *MultiStreamInputSession) Initialize(in MultiStreamContextMessage) error {
	if in.Text == "" {
		in.Text = " "
	}
	return s.send(in)
}

// InitializeContext initializes or re-initializes a multi-context WebSocket
// context.
func (s *MultiStreamInputSession) InitializeContext(in MultiStreamContextMessage) error {
	return s.send(in)
}

// SendText sends text to a multi-context WebSocket context.
func (s *MultiStreamInputSession) SendText(in MultiStreamTextMessage) error {
	return s.send(in)
}

// FlushContext flushes one multi-context WebSocket context.
func (s *MultiStreamInputSession) FlushContext(in MultiStreamFlushMessage) error {
	if strings.TrimSpace(in.ContextID) == "" {
		return errors.New("elevenlabs: context_id is required")
	}
	in.Flush = true
	return s.send(in)
}

// CloseContext closes one multi-context WebSocket context.
func (s *MultiStreamInputSession) CloseContext(contextID string) error {
	if strings.TrimSpace(contextID) == "" {
		return errors.New("elevenlabs: context_id is required")
	}
	return s.send(struct {
		ContextID    string `json:"context_id"`
		CloseContext bool   `json:"close_context"`
	}{
		ContextID:    contextID,
		CloseContext: true,
	})
}

// CloseSocket asks the server to close all contexts and the WebSocket
// connection gracefully.
func (s *MultiStreamInputSession) CloseSocket() error {
	return s.send(struct {
		CloseSocket bool `json:"close_socket"`
	}{
		CloseSocket: true,
	})
}

// KeepContextAlive resets the inactivity timeout for one context.
func (s *MultiStreamInputSession) KeepContextAlive(contextID string) error {
	if strings.TrimSpace(contextID) == "" {
		return errors.New("elevenlabs: context_id is required")
	}
	return s.send(struct {
		Text      string `json:"text"`
		ContextID string `json:"context_id"`
	}{
		Text:      "",
		ContextID: contextID,
	})
}

// Receive reads one event from a multi-context WebSocket stream.
func (s *MultiStreamInputSession) Receive() (*StreamInputEvent, error) {
	if s == nil || s.conn == nil {
		return nil, errors.New("elevenlabs: nil multi text-to-speech stream session")
	}
	var event StreamInputEvent
	if err := websocket.JSON.Receive(s.conn, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// Close closes the multi-context WebSocket stream.
func (s *MultiStreamInputSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *MultiStreamInputSession) send(v any) error {
	if s == nil || s.conn == nil {
		return errors.New("elevenlabs: nil multi text-to-speech stream session")
	}
	return websocket.JSON.Send(s.conn, v)
}
