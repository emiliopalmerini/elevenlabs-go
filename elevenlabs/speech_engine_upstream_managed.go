package elevenlabs

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrSpeechEngineStaleResponse is returned when a transcript response tries to
// write after a newer transcript event has taken over the session.
var ErrSpeechEngineStaleResponse = errors.New("elevenlabs: stale speech engine response")

// SpeechEngineUpstreamHandlers contains callbacks for the managed upstream
// handler adapter.
type SpeechEngineUpstreamHandlers struct {
	OnInit       func(context.Context, string, *SpeechEngineUpstreamSession) error
	OnTranscript func(context.Context, *SpeechEngineTranscriptResponse) error
	OnClose      func(context.Context, *SpeechEngineUpstreamSession) error
	OnError      func(context.Context, string, *SpeechEngineUpstreamSession) error
}

// SpeechEngineTranscriptResponse is passed to managed transcript handlers. It
// sends chunks for the transcript event it represents and rejects stale writes
// after a newer transcript arrives.
type SpeechEngineTranscriptResponse struct {
	EventID        int64
	UserTranscript []SpeechEngineTranscriptMessage
	Session        *SpeechEngineUpstreamSession

	isCurrent func(int64) bool
}

// SendText sends one complete text response followed by the required final
// empty response.
func (r *SpeechEngineTranscriptResponse) SendText(ctx context.Context, content string) error {
	if err := r.SendAgentResponse(ctx, content, false); err != nil {
		return err
	}
	return r.SendFinal(ctx)
}

// SendFinal sends the required final empty response for the transcript event.
func (r *SpeechEngineTranscriptResponse) SendFinal(ctx context.Context) error {
	return r.SendAgentResponse(ctx, "", true)
}

// SendAgentResponse sends one text chunk for the transcript event.
func (r *SpeechEngineTranscriptResponse) SendAgentResponse(ctx context.Context, content string, isFinal bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if r == nil || r.Session == nil {
		return errors.New("elevenlabs: nil speech engine transcript response")
	}
	if r.isCurrent != nil && !r.isCurrent(r.EventID) {
		return ErrSpeechEngineStaleResponse
	}
	return r.Session.SendAgentResponse(r.EventID, content, isFinal)
}

// NewManagedSpeechEngineUpstreamHandler adapts callbacks into a
// SpeechEngineUpstreamHandler. It auto-responds to pings and cancels the
// previous transcript context whenever a newer transcript arrives.
func NewManagedSpeechEngineUpstreamHandler(handlers SpeechEngineUpstreamHandlers) SpeechEngineUpstreamHandler {
	return func(ctx context.Context, session *SpeechEngineUpstreamSession) error {
		if ctx == nil {
			ctx = context.Background()
		}

		state := &managedSpeechEngineState{}
		errCh := make(chan error, 1)
		reportHandlerError := func(err error) {
			if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, ErrSpeechEngineStaleResponse) {
				return
			}
			select {
			case errCh <- err:
			default:
			}
		}
		defer state.cancelCurrent()

		for {
			select {
			case err := <-errCh:
				state.cancelCurrent()
				return err
			default:
			}

			message, err := session.Receive()
			if err != nil {
				state.cancelCurrent()
				return err
			}

			switch message.Type {
			case SpeechEngineMessageInit:
				if handlers.OnInit != nil {
					if err := handlers.OnInit(ctx, message.ConversationID, session); err != nil {
						state.cancelCurrent()
						return err
					}
				}
			case SpeechEngineMessagePing:
				if err := session.SendPong(); err != nil {
					state.cancelCurrent()
					return err
				}
			case SpeechEngineMessageUserTranscript:
				transcriptCtx := state.replaceCurrent(ctx, message.EventID)
				response := &SpeechEngineTranscriptResponse{
					EventID:        message.EventID,
					UserTranscript: append([]SpeechEngineTranscriptMessage(nil), message.UserTranscript...),
					Session:        session,
					isCurrent:      state.isCurrent,
				}
				if handlers.OnTranscript != nil {
					go func() {
						reportHandlerError(handlers.OnTranscript(transcriptCtx, response))
					}()
				}
			case SpeechEngineMessageClose:
				state.cancelCurrent()
				if handlers.OnClose != nil {
					return handlers.OnClose(ctx, session)
				}
				return nil
			case SpeechEngineMessageError:
				state.cancelCurrent()
				if handlers.OnError != nil {
					return handlers.OnError(ctx, message.Message, session)
				}
				return fmt.Errorf("elevenlabs: speech engine upstream error: %s", message.Message)
			}
		}
	}
}

type managedSpeechEngineState struct {
	mu             sync.Mutex
	currentEventID int64
	hasCurrent     bool
	cancel         context.CancelFunc
}

func (s *managedSpeechEngineState) replaceCurrent(parent context.Context, eventID int64) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)
	var previous context.CancelFunc

	s.mu.Lock()
	previous = s.cancel
	s.currentEventID = eventID
	s.hasCurrent = true
	s.cancel = cancel
	s.mu.Unlock()

	if previous != nil {
		previous()
	}
	return ctx
}

func (s *managedSpeechEngineState) cancelCurrent() {
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.hasCurrent = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (s *managedSpeechEngineState) isCurrent(eventID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasCurrent && s.currentEventID == eventID
}
