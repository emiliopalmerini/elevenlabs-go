package elevenlabs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestSpeechEngineUpstreamServerRejectsUnauthorizedRequest(t *testing.T) {
	var handlerCalled atomic.Bool
	errorCh := make(chan error, 1)

	server := httptest.NewServer(SpeechEngineUpstreamServer{
		APIKey: "test-key",
		Handler: func(context.Context, *SpeechEngineUpstreamSession) error {
			handlerCalled.Store(true)
			return nil
		},
		OnError: func(_ *http.Request, err error) {
			errorCh <- err
		},
	})
	defer server.Close()

	resp, err := server.Client().Get(server.URL)
	if err != nil {
		t.Fatalf("GET returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if handlerCalled.Load() {
		t.Fatal("handler was called for unauthorized request")
	}

	select {
	case err := <-errorCh:
		if err == nil || !strings.Contains(err.Error(), "token is required") {
			t.Fatalf("reported error = %v, want missing token error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("OnError was not called")
	}
}

func TestSpeechEngineUpstreamServerAcceptsAuthorizedWebSocket(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	handlerDone := make(chan error, 1)
	errorCh := make(chan error, 1)

	server := httptest.NewServer(SpeechEngineUpstreamServer{
		APIKey: "test-key",
		Handler: func(ctx context.Context, session *SpeechEngineUpstreamSession) error {
			err := testAuthorizedSpeechEngineUpstreamHandler(ctx, session)
			handlerDone <- err
			return err
		},
		Now: func() time.Time {
			return now
		},
		OnError: func(_ *http.Request, err error) {
			errorCh <- err
		},
	})
	defer server.Close()

	token := speechEngineTestJWT(t, "test-key", map[string]any{
		"alg": "HS256",
	}, map[string]any{
		"exp": now.Add(time.Minute).Unix(),
		"iss": speechEngineJWTIssuer,
		"sub": speechEngineJWTSubject,
	})

	conn, err := speechEngineDialAuthorizedWebSocket(server.URL, token)
	if err != nil {
		t.Fatalf("dial authorized websocket: %v", err)
	}
	defer conn.Close()

	if err := websocket.JSON.Send(conn, SpeechEngineUpstreamMessage{
		ConversationID: "conv_123",
		Type:           SpeechEngineMessageInit,
	}); err != nil {
		t.Fatalf("send init: %v", err)
	}

	var pong SpeechEnginePong
	if err := websocket.JSON.Receive(conn, &pong); err != nil {
		t.Fatalf("receive pong: %v", err)
	}
	if pong.Type != SpeechEngineMessagePong {
		t.Fatalf("pong type = %q, want %q", pong.Type, SpeechEngineMessagePong)
	}

	select {
	case err := <-handlerDone:
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not finish")
	}

	select {
	case err := <-errorCh:
		t.Fatalf("OnError called unexpectedly: %v", err)
	default:
	}
}

func TestSpeechEngineUpstreamServerRejectsNonGET(t *testing.T) {
	server := httptest.NewServer(SpeechEngineUpstreamServer{
		APIKey: "test-key",
		Handler: func(context.Context, *SpeechEngineUpstreamSession) error {
			return nil
		},
	})
	defer server.Close()

	resp, err := server.Client().Post(server.URL, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
	if resp.Header.Get("Allow") != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", resp.Header.Get("Allow"))
	}
}

func testAuthorizedSpeechEngineUpstreamHandler(ctx context.Context, session *SpeechEngineUpstreamSession) error {
	if ctx == nil {
		return errors.New("nil context")
	}

	message, err := session.Receive()
	if err != nil {
		return fmt.Errorf("receive init: %w", err)
	}
	if message.ConversationID != "conv_123" || message.Type != SpeechEngineMessageInit {
		return fmt.Errorf("message = %+v, want conversation init", message)
	}
	if err := session.SendPong(); err != nil {
		return fmt.Errorf("send pong: %w", err)
	}

	return nil
}

func speechEngineDialAuthorizedWebSocket(httpURL, token string) (*websocket.Conn, error) {
	config, err := websocket.NewConfig(speechEngineTestWebSocketURL(httpURL), httpURL)
	if err != nil {
		return nil, err
	}
	config.Header.Set(SpeechEngineAuthorizationHeader, token)
	return config.DialContext(context.Background())
}
