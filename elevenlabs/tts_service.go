package elevenlabs

import "errors"

// TTSService provides ElevenLabs text-to-speech APIs.
type TTSService struct {
	client *Client
}

func (s *TTSService) apiClient() (*Client, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	return s.client, nil
}
