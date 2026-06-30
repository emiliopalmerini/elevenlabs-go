package elevenlabs

import (
	"errors"
	"sync"

	"golang.org/x/net/websocket"
)

// SpeechEngineUpstreamSession is one accepted Speech Engine upstream WebSocket
// connection from ElevenLabs.
type SpeechEngineUpstreamSession struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func newSpeechEngineUpstreamSession(conn *websocket.Conn) *SpeechEngineUpstreamSession {
	return &SpeechEngineUpstreamSession{conn: conn}
}

// Close closes the Speech Engine upstream WebSocket connection.
func (s *SpeechEngineUpstreamSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

// Receive reads one message from the Speech Engine upstream WebSocket
// connection.
func (s *SpeechEngineUpstreamSession) Receive() (*SpeechEngineUpstreamMessage, error) {
	if s == nil || s.conn == nil {
		return nil, errors.New("elevenlabs: nil speech engine upstream session")
	}
	var message SpeechEngineUpstreamMessage
	if err := websocket.JSON.Receive(s.conn, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

// SendAgentResponse sends one text chunk to ElevenLabs for synthesis.
func (s *SpeechEngineUpstreamSession) SendAgentResponse(eventID int64, content string, isFinal bool) error {
	return s.send(SpeechEngineAgentResponse{
		Content: content,
		EventID: eventID,
		IsFinal: isFinal,
		Type:    SpeechEngineMessageAgentResponse,
	})
}

// SendPong replies to a Speech Engine ping message.
func (s *SpeechEngineUpstreamSession) SendPong() error {
	return s.send(SpeechEnginePong{Type: SpeechEngineMessagePong})
}

func (s *SpeechEngineUpstreamSession) send(v any) error {
	if s == nil || s.conn == nil {
		return errors.New("elevenlabs: nil speech engine upstream session")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return websocket.JSON.Send(s.conn, v)
}
