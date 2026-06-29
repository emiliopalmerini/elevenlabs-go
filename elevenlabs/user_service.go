package elevenlabs

import "errors"

// UserService provides ElevenLabs authenticated user APIs.
type UserService struct {
	client *Client
}

func (s *UserService) apiClient() (*Client, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	return s.client, nil
}
