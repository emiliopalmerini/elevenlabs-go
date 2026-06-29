package elevenlabs

import "errors"

// STTService provides ElevenLabs speech-to-text APIs.
type STTService struct {
	client *Client
}

func (s *STTService) apiClient() (*Client, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	return s.client, nil
}
