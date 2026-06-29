package elevenlabs

import (
	"bytes"
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
	apiKey      string
	baseURL     string
	httpClient  *http.Client
	userAgent   string
	retryConfig retryConfig
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// RequestBuilder builds an HTTP request for one attempt.
type RequestBuilder func(context.Context) (*http.Request, error)

// NewClient creates a Client that authenticates with apiKey.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:      apiKey,
		baseURL:     defaultBaseURL,
		httpClient:  http.DefaultClient,
		userAgent:   defaultUserAgent,
		retryConfig: defaultRetryConfig(),
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

// NewRequest creates an authenticated API request against the configured base URL.
func (c *Client) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	return c.newRequest(ctx, method, path, body)
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

// Endpoint resolves an API path against the configured base URL.
func (c *Client) Endpoint(path string) (string, error) {
	return c.endpoint(path)
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

// AuthHeader returns the API key auth header for non-standard transports.
func (c *Client) AuthHeader() (http.Header, error) {
	if c == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, errors.New("elevenlabs: api key is required")
	}

	header := http.Header{}
	header.Set("xi-api-key", c.apiKey)
	return header, nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) (RawResponse, error) {
	return c.GetJSON(ctx, path, out)
}

// GetJSON sends a retryable GET request and decodes the JSON response.
func (c *Client) GetJSON(ctx context.Context, path string, out any) (RawResponse, error) {
	build := func(ctx context.Context) (*http.Request, error) {
		return c.newRequest(ctx, http.MethodGet, path, nil)
	}

	body, raw, err := c.do(ctx, build, true)
	if err != nil {
		return raw, err
	}
	if err := decodeResponse(body, out); err != nil {
		return raw, err
	}
	return raw, nil
}

func (c *Client) do(ctx context.Context, build RequestBuilder, retryable bool) ([]byte, RawResponse, error) {
	return c.Do(ctx, build, retryable)
}

// Do executes requests with the client's retry policy.
func (c *Client) Do(ctx context.Context, build RequestBuilder, retryable bool) ([]byte, RawResponse, error) {
	if c == nil {
		return nil, RawResponse{}, errors.New("elevenlabs: nil client")
	}

	cfg := c.retryConfig
	if cfg.maxAttempts == 0 {
		cfg = defaultRetryConfig()
	}

	maxAttempts := 1
	if retryable {
		maxAttempts = cfg.maxAttempts
	}

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, RawResponse{}, err
		}

		req, err := build(ctx)
		if err != nil {
			return nil, RawResponse{}, err
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, RawResponse{}, ctxErr
			}
			if attempt == maxAttempts {
				return nil, RawResponse{}, err
			}
			if err := waitForRetry(ctx, cfg.backoffDelay(attempt)); err != nil {
				return nil, RawResponse{}, err
			}
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			body, raw, err := readResponse(resp)
			return body, raw, err
		}

		retryAfter := resp.Header.Get("Retry-After")
		apiErr, raw, readErr := readAPIError(resp)
		if readErr != nil {
			if attempt < maxAttempts && cfg.retryableStatus(resp.StatusCode) {
				if err := waitForRetry(ctx, cfg.retryDelay(attempt, retryAfter)); err != nil {
					return nil, raw, err
				}
				continue
			}
			return nil, raw, readErr
		}

		if attempt == maxAttempts || !cfg.retryableStatus(apiErr.StatusCode) {
			return nil, raw, apiErr
		}
		if err := waitForRetry(ctx, cfg.retryDelay(attempt, retryAfter)); err != nil {
			return nil, raw, err
		}
	}

	return nil, RawResponse{}, nil
}

func readResponse(resp *http.Response) ([]byte, RawResponse, error) {
	defer resp.Body.Close()

	raw := newRawResponse(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, raw, fmt.Errorf("elevenlabs: read response: %w", err)
	}
	return body, raw, nil
}

func decodeResponse(body []byte, out any) error {
	return DecodeResponse(body, out)
}

// DecodeResponse decodes a JSON response body into out.
func DecodeResponse(body []byte, out any) error {
	if out == nil {
		return nil
	}

	if err := json.NewDecoder(bytes.NewReader(body)).Decode(out); err != nil {
		return fmt.Errorf("elevenlabs: decode response: %w", err)
	}

	return nil
}

func decodeOptionalResponse(body []byte) (any, error) {
	return DecodeOptionalResponse(body)
}

// DecodeOptionalResponse decodes a JSON response body when one is present.
func DecodeOptionalResponse(body []byte) (any, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, nil
	}
	var out any
	if err := decodeResponse(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func readAPIError(resp *http.Response) (*APIError, RawResponse, error) {
	defer resp.Body.Close()

	raw := newRawResponse(resp)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, raw, fmt.Errorf("elevenlabs: read error response: %w", err)
	}

	return newAPIError(resp.StatusCode, resp.Status, body, raw), raw, nil
}
