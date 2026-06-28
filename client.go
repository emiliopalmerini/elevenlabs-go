package elevenlabs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultBaseURL   = "https://api.elevenlabs.io"
	defaultUserAgent = "elevenlabs-go"
)

// Client is an ElevenLabs API client.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// NewClient creates a Client that authenticates with apiKey.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: http.DefaultClient,
		userAgent:  defaultUserAgent,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}

	return c
}

// WithBaseURL overrides the ElevenLabs API base URL.
//
// This is primarily useful for tests.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient overrides the HTTP client used for requests.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if c == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, errors.New("elevenlabs: api key is required")
	}

	endpoint, err := c.endpoint(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("xi-api-key", c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)

	return req, nil
}

func (c *Client) endpoint(path string) (string, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("elevenlabs: parse base url: %w", err)
	}
	if !base.IsAbs() {
		return "", fmt.Errorf("elevenlabs: base url must be absolute: %q", c.baseURL)
	}

	relative, err := url.Parse(strings.TrimPrefix(path, "/"))
	if err != nil {
		return "", fmt.Errorf("elevenlabs: parse request path: %w", err)
	}

	return base.ResolveReference(relative).String(), nil
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("elevenlabs: read error response: %w", readErr)
		}
		return newAPIError(resp.StatusCode, resp.Status, body)
	}

	if out == nil {
		_, err = io.Copy(io.Discard, resp.Body)
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("elevenlabs: decode response: %w", err)
	}

	return nil
}
