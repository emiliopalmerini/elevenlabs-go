package elevenlabs

const (
	// SpeechEngineAuthorizationHeader is sent by ElevenLabs on upstream
	// Speech Engine WebSocket upgrade requests.
	SpeechEngineAuthorizationHeader = "X-Elevenlabs-Speech-Engine-Authorization"
)

// SpeechEngineUpstreamMessageType identifies messages sent by ElevenLabs to an
// upstream Speech Engine server.
type SpeechEngineUpstreamMessageType string

const (
	SpeechEngineMessageInit           SpeechEngineUpstreamMessageType = "init"
	SpeechEngineMessageUserTranscript SpeechEngineUpstreamMessageType = "user_transcript"
	SpeechEngineMessagePing           SpeechEngineUpstreamMessageType = "ping"
	SpeechEngineMessageClose          SpeechEngineUpstreamMessageType = "close"
	SpeechEngineMessageError          SpeechEngineUpstreamMessageType = "error"
)

// SpeechEngineTranscriptRole identifies the speaker for a transcript turn.
type SpeechEngineTranscriptRole string

const (
	SpeechEngineTranscriptRoleUser  SpeechEngineTranscriptRole = "user"
	SpeechEngineTranscriptRoleAgent SpeechEngineTranscriptRole = "agent"
)

// SpeechEngineTranscriptMessage is one turn in the conversation history sent
// by ElevenLabs.
type SpeechEngineTranscriptMessage struct {
	Content string                     `json:"content"`
	Role    SpeechEngineTranscriptRole `json:"role"`
}

// SpeechEngineUpstreamMessage is a message sent by ElevenLabs to an upstream
// Speech Engine server.
type SpeechEngineUpstreamMessage struct {
	ConversationID string                          `json:"conversation_id,omitempty"`
	EventID        int64                           `json:"event_id,omitempty"`
	Message        string                          `json:"message,omitempty"`
	Type           SpeechEngineUpstreamMessageType `json:"type"`
	UserTranscript []SpeechEngineTranscriptMessage `json:"user_transcript,omitempty"`
}

// SpeechEngineDownstreamMessageType identifies messages sent by an upstream
// Speech Engine server back to ElevenLabs.
type SpeechEngineDownstreamMessageType string

const (
	SpeechEngineMessageAgentResponse SpeechEngineDownstreamMessageType = "agent_response"
	SpeechEngineMessagePong          SpeechEngineDownstreamMessageType = "pong"
)

// SpeechEngineAgentResponse is a text chunk sent to ElevenLabs for synthesis.
type SpeechEngineAgentResponse struct {
	Content string                            `json:"content"`
	EventID int64                             `json:"event_id,omitempty"`
	IsFinal bool                              `json:"is_final"`
	Type    SpeechEngineDownstreamMessageType `json:"type"`
}

// SpeechEnginePong replies to a Speech Engine ping message.
type SpeechEnginePong struct {
	Type SpeechEngineDownstreamMessageType `json:"type"`
}
