package elevenlabs

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestManagedSpeechEngineUpstreamHandlerAutoPongsAndDispatchesInit(t *testing.T) {
	initCh := make(chan string, 1)
	handlerErr := make(chan error, 1)

	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		handler := NewManagedSpeechEngineUpstreamHandler(SpeechEngineUpstreamHandlers{
			OnInit: func(_ context.Context, conversationID string, _ *SpeechEngineUpstreamSession) error {
				initCh <- conversationID
				return nil
			},
		})
		handlerErr <- handler(context.Background(), newSpeechEngineUpstreamSession(ws))
	}))
	defer server.Close()

	conn, err := websocket.Dial(speechEngineTestWebSocketURL(server.URL), "", server.URL)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := websocket.JSON.Send(conn, SpeechEngineUpstreamMessage{
		ConversationID: "conv_123",
		Type:           SpeechEngineMessageInit,
	}); err != nil {
		t.Fatalf("send init: %v", err)
	}
	if err := websocket.JSON.Send(conn, SpeechEngineUpstreamMessage{
		Type: SpeechEngineMessagePing,
	}); err != nil {
		t.Fatalf("send ping: %v", err)
	}

	select {
	case got := <-initCh:
		if got != "conv_123" {
			t.Fatalf("conversation id = %q, want conv_123", got)
		}
	case <-time.After(time.Second):
		t.Fatal("OnInit was not called")
	}

	var pong SpeechEnginePong
	if err := websocket.JSON.Receive(conn, &pong); err != nil {
		t.Fatalf("receive pong: %v", err)
	}
	if pong.Type != SpeechEngineMessagePong {
		t.Fatalf("pong = %#v, want pong", pong)
	}

	if err := websocket.JSON.Send(conn, SpeechEngineUpstreamMessage{Type: SpeechEngineMessageClose}); err != nil {
		t.Fatalf("send close: %v", err)
	}
	select {
	case err := <-handlerErr:
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not finish")
	}
}

func TestManagedSpeechEngineUpstreamHandlerSendsTranscriptResponse(t *testing.T) {
	handlerErr := make(chan error, 1)

	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		handler := NewManagedSpeechEngineUpstreamHandler(SpeechEngineUpstreamHandlers{
			OnTranscript: func(ctx context.Context, response *SpeechEngineTranscriptResponse) error {
				if response.EventID != 42 {
					return fmt.Errorf("event id = %d, want 42", response.EventID)
				}
				if len(response.UserTranscript) != 1 || response.UserTranscript[0].Content != "Hello" {
					return fmt.Errorf("transcript = %#v, want Hello", response.UserTranscript)
				}
				return response.SendText(ctx, "Hello from agent")
			},
		})
		handlerErr <- handler(context.Background(), newSpeechEngineUpstreamSession(ws))
	}))
	defer server.Close()

	conn, err := websocket.Dial(speechEngineTestWebSocketURL(server.URL), "", server.URL)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := websocket.JSON.Send(conn, SpeechEngineUpstreamMessage{
		EventID: 42,
		Type:    SpeechEngineMessageUserTranscript,
		UserTranscript: []SpeechEngineTranscriptMessage{
			{Role: SpeechEngineTranscriptRoleUser, Content: "Hello"},
		},
	}); err != nil {
		t.Fatalf("send transcript: %v", err)
	}

	var chunk SpeechEngineAgentResponse
	if err := websocket.JSON.Receive(conn, &chunk); err != nil {
		t.Fatalf("receive agent chunk: %v", err)
	}
	if chunk.Content != "Hello from agent" || chunk.EventID != 42 || chunk.IsFinal {
		t.Fatalf("chunk = %#v, want non-final event 42 response", chunk)
	}

	var final SpeechEngineAgentResponse
	if err := websocket.JSON.Receive(conn, &final); err != nil {
		t.Fatalf("receive final response: %v", err)
	}
	if final.Content != "" || final.EventID != 42 || !final.IsFinal {
		t.Fatalf("final = %#v, want final event 42 response", final)
	}

	if err := websocket.JSON.Send(conn, SpeechEngineUpstreamMessage{Type: SpeechEngineMessageClose}); err != nil {
		t.Fatalf("send close: %v", err)
	}
	select {
	case err := <-handlerErr:
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not finish")
	}
}

func TestManagedSpeechEngineUpstreamHandlerCancelsStaleTranscriptResponse(t *testing.T) {
	firstStarted := make(chan struct{}, 1)
	staleErr := make(chan error, 1)
	handlerErr := make(chan error, 1)

	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		handler := NewManagedSpeechEngineUpstreamHandler(SpeechEngineUpstreamHandlers{
			OnTranscript: func(ctx context.Context, response *SpeechEngineTranscriptResponse) error {
				switch response.EventID {
				case 1:
					firstStarted <- struct{}{}
					<-ctx.Done()
					staleErr <- response.SendAgentResponse(ctx, "stale", false)
					return nil
				case 2:
					return response.SendText(ctx, "fresh")
				default:
					return fmt.Errorf("unexpected event id %d", response.EventID)
				}
			},
		})
		handlerErr <- handler(context.Background(), newSpeechEngineUpstreamSession(ws))
	}))
	defer server.Close()

	conn, err := websocket.Dial(speechEngineTestWebSocketURL(server.URL), "", server.URL)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := websocket.JSON.Send(conn, SpeechEngineUpstreamMessage{
		EventID:        1,
		Type:           SpeechEngineMessageUserTranscript,
		UserTranscript: []SpeechEngineTranscriptMessage{{Role: SpeechEngineTranscriptRoleUser, Content: "first"}},
	}); err != nil {
		t.Fatalf("send first transcript: %v", err)
	}
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first transcript handler did not start")
	}

	if err := websocket.JSON.Send(conn, SpeechEngineUpstreamMessage{
		EventID:        2,
		Type:           SpeechEngineMessageUserTranscript,
		UserTranscript: []SpeechEngineTranscriptMessage{{Role: SpeechEngineTranscriptRoleUser, Content: "second"}},
	}); err != nil {
		t.Fatalf("send second transcript: %v", err)
	}

	var chunk SpeechEngineAgentResponse
	if err := websocket.JSON.Receive(conn, &chunk); err != nil {
		t.Fatalf("receive fresh chunk: %v", err)
	}
	if chunk.Content != "fresh" || chunk.EventID != 2 || chunk.IsFinal {
		t.Fatalf("chunk = %#v, want fresh event 2 response", chunk)
	}

	var final SpeechEngineAgentResponse
	if err := websocket.JSON.Receive(conn, &final); err != nil {
		t.Fatalf("receive fresh final: %v", err)
	}
	if final.Content != "" || final.EventID != 2 || !final.IsFinal {
		t.Fatalf("final = %#v, want final event 2 response", final)
	}

	select {
	case err := <-staleErr:
		if !errors.Is(err, context.Canceled) && !errors.Is(err, ErrSpeechEngineStaleResponse) {
			t.Fatalf("stale send error = %v, want cancellation or stale response", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stale response was not cancelled")
	}

	if err := websocket.JSON.Send(conn, SpeechEngineUpstreamMessage{Type: SpeechEngineMessageClose}); err != nil {
		t.Fatalf("send close: %v", err)
	}
	select {
	case err := <-handlerErr:
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not finish")
	}
}
