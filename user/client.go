package user

import (
	"errors"

	elevenlabs "github.com/emiliopalmerini/elevenlabs-go"
)

// Client provides ElevenLabs authenticated user APIs.
type Client struct {
	core *elevenlabs.Client
}

// NewClient creates a user client that authenticates with apiKey.
func NewClient(apiKey string, opts ...elevenlabs.ClientOption) *Client {
	return New(elevenlabs.NewClient(apiKey, opts...))
}

// New creates a user client from a shared ElevenLabs client.
func New(core *elevenlabs.Client) *Client {
	return &Client{core: core}
}

// Core returns the underlying shared ElevenLabs client.
func (c *Client) Core() *elevenlabs.Client {
	if c == nil {
		return nil
	}
	return c.core
}

func (c *Client) apiClient() (*elevenlabs.Client, error) {
	if c == nil || c.core == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	return c.core, nil
}
