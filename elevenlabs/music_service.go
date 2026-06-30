package elevenlabs

import "errors"

// MusicService provides ElevenLabs music generation APIs.
type MusicService struct {
	client *Client
}

func (s *MusicService) apiClient() (*Client, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	return s.client, nil
}
