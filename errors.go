package elevenlabs

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIError is returned when the ElevenLabs API responds with a non-2xx status.
type APIError struct {
	StatusCode int
	Status     string
	Message    string
	Body       []byte
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("elevenlabs: api error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("elevenlabs: api error %d", e.StatusCode)
}

func newAPIError(statusCode int, status string, body []byte) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Status:     status,
		Message:    errorMessage(body),
		Body:       body,
	}
}

func errorMessage(body []byte) string {
	var payload struct {
		Detail  any    `json:"detail"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return strings.TrimSpace(string(body))
	}

	switch detail := payload.Detail.(type) {
	case string:
		if detail != "" {
			return detail
		}
	case map[string]any:
		if message, ok := detail["message"].(string); ok && message != "" {
			return message
		}
	}

	if payload.Message != "" {
		return payload.Message
	}
	if payload.Error != "" {
		return payload.Error
	}

	return strings.TrimSpace(string(body))
}
