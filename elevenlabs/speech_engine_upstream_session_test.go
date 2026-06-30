package elevenlabs

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/websocket"
)

func TestSpeechEngineUpstreamSessionReceivesAndSendsMessages(t *testing.T) {
	handlerErr := make(chan error, 1)
	server := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		handlerErr <- testSpeechEngineUpstreamSession(ws)
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
		EventID: 42,
		Type:    SpeechEngineMessageUserTranscript,
		UserTranscript: []SpeechEngineTranscriptMessage{
			{
				Content: "Hello",
				Role:    SpeechEngineTranscriptRoleUser,
			},
			{
				Content: "Hi there",
				Role:    SpeechEngineTranscriptRoleAgent,
			},
		},
	}); err != nil {
		t.Fatalf("send user transcript: %v", err)
	}

	var pong SpeechEnginePong
	if err := websocket.JSON.Receive(conn, &pong); err != nil {
		t.Fatalf("receive pong: %v", err)
	}
	if pong.Type != SpeechEngineMessagePong {
		t.Fatalf("pong type = %q, want %q", pong.Type, SpeechEngineMessagePong)
	}

	var chunk SpeechEngineAgentResponse
	if err := websocket.JSON.Receive(conn, &chunk); err != nil {
		t.Fatalf("receive agent chunk: %v", err)
	}
	if chunk.Content != "Hello " || chunk.EventID != 42 || chunk.IsFinal || chunk.Type != SpeechEngineMessageAgentResponse {
		t.Fatalf("chunk = %+v, want non-final response for event 42", chunk)
	}

	var final SpeechEngineAgentResponse
	if err := websocket.JSON.Receive(conn, &final); err != nil {
		t.Fatalf("receive final response: %v", err)
	}
	if final.Content != "" || final.EventID != 42 || !final.IsFinal || final.Type != SpeechEngineMessageAgentResponse {
		t.Fatalf("final = %+v, want final empty response for event 42", final)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("close websocket: %v", err)
	}
	if err := <-handlerErr; err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
}

func TestSpeechEngineUpstreamSessionRejectsNil(t *testing.T) {
	var session *SpeechEngineUpstreamSession

	if err := session.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if _, err := session.Receive(); err == nil {
		t.Fatal("Receive returned nil error")
	}
	if err := session.SendAgentResponse(1, "Hello", false); err == nil {
		t.Fatal("SendAgentResponse returned nil error")
	}
	if err := session.SendPong(); err == nil {
		t.Fatal("SendPong returned nil error")
	}
}

func testSpeechEngineUpstreamSession(ws *websocket.Conn) error {
	session := newSpeechEngineUpstreamSession(ws)

	init, err := session.Receive()
	if err != nil {
		return fmt.Errorf("receive init: %w", err)
	}
	if init.ConversationID != "conv_123" || init.Type != SpeechEngineMessageInit {
		return fmt.Errorf("init = %+v, want conversation init", init)
	}

	transcript, err := session.Receive()
	if err != nil {
		return fmt.Errorf("receive transcript: %w", err)
	}
	if transcript.EventID != 42 || transcript.Type != SpeechEngineMessageUserTranscript {
		return fmt.Errorf("transcript = %+v, want user transcript event 42", transcript)
	}
	if len(transcript.UserTranscript) != 2 {
		return fmt.Errorf("transcript length = %d, want 2", len(transcript.UserTranscript))
	}
	if transcript.UserTranscript[0].Content != "Hello" || transcript.UserTranscript[0].Role != SpeechEngineTranscriptRoleUser {
		return fmt.Errorf("first transcript turn = %+v, want user Hello", transcript.UserTranscript[0])
	}
	if transcript.UserTranscript[1].Content != "Hi there" || transcript.UserTranscript[1].Role != SpeechEngineTranscriptRoleAgent {
		return fmt.Errorf("second transcript turn = %+v, want agent Hi there", transcript.UserTranscript[1])
	}

	if err := session.SendPong(); err != nil {
		return fmt.Errorf("send pong: %w", err)
	}
	if err := session.SendAgentResponse(transcript.EventID, "Hello ", false); err != nil {
		return fmt.Errorf("send agent chunk: %w", err)
	}
	if err := session.SendAgentResponse(transcript.EventID, "", true); err != nil {
		return fmt.Errorf("send final response: %w", err)
	}

	return nil
}

func speechEngineTestWebSocketURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}
