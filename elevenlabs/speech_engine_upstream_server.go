package elevenlabs

import (
	"context"
	"errors"
	"net/http"
	"time"

	"golang.org/x/net/websocket"
)

// SpeechEngineUpstreamHandler handles one accepted Speech Engine upstream
// WebSocket connection.
type SpeechEngineUpstreamHandler func(context.Context, *SpeechEngineUpstreamSession) error

// SpeechEngineUpstreamServer verifies and accepts Speech Engine upstream
// WebSocket connections from ElevenLabs.
type SpeechEngineUpstreamServer struct {
	APIKey  string
	Handler SpeechEngineUpstreamHandler
	Now     func() time.Time
	OnError func(*http.Request, error)
}

// ServeHTTP verifies the Speech Engine authorization JWT and upgrades valid
// requests to a WebSocket connection.
func (s SpeechEngineUpstreamServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		err := errors.New("elevenlabs: speech engine upstream requires GET")
		s.reportError(r, err)
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if s.Handler == nil {
		err := errors.New("elevenlabs: speech engine upstream handler is required")
		s.reportError(r, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	if err := VerifySpeechEngineAuthorization(r.Header.Get(SpeechEngineAuthorizationHeader), s.APIKey, now); err != nil {
		s.reportError(r, err)
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	server := websocket.Server{
		Handler: func(conn *websocket.Conn) {
			session := newSpeechEngineUpstreamSession(conn)
			defer session.Close()

			if err := s.Handler(r.Context(), session); err != nil {
				s.reportError(r, err)
			}
		},
	}
	server.ServeHTTP(w, r)
}

func (s SpeechEngineUpstreamServer) reportError(r *http.Request, err error) {
	if err == nil || s.OnError == nil {
		return
	}
	s.OnError(r, err)
}
