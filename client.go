package elevenlabs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultUserAgent = "elevenlabs-go/0.1"

// Client is an ElevenLabs API client.
type Client struct {
	apiKey      string
	baseURL     *url.URL
	httpClient  *http.Client
	userAgent   string
	retryPolicy RetryPolicy

	SpeechToText *SpeechToTextService
	User         *UserService
	Models       *ModelsService
}

type clientConfig struct {
	apiKey      string
	baseURL     string
	httpClient  *http.Client
	userAgent   string
	retryPolicy RetryPolicy
}

// Option configures a Client.
type Option func(*clientConfig) error

// RetryPolicy controls optional request retries. The default client does not
// retry requests. Set MaxAttempts above 1 to opt in.
type RetryPolicy struct {
	MaxAttempts           int
	BaseDelay             time.Duration
	MaxDelay              time.Duration
	RetryRateLimits       bool
	RetryConnectionErrors bool
}

// DefaultRetryPolicy returns a conservative retry policy suitable for callers
// that explicitly want retries. Upload requests are only retried when their body
// can be rebuilt.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:           3,
		BaseDelay:             200 * time.Millisecond,
		MaxDelay:              2 * time.Second,
		RetryRateLimits:       true,
		RetryConnectionErrors: true,
	}
}

// WithAPIKey sets the ElevenLabs API key.
func WithAPIKey(apiKey string) Option {
	return func(cfg *clientConfig) error {
		cfg.apiKey = strings.TrimSpace(apiKey)
		return nil
	}
}

// WithBaseURL overrides the API origin. It is useful for residency endpoints
// and tests.
func WithBaseURL(baseURL string) Option {
	return func(cfg *clientConfig) error {
		cfg.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		return nil
	}
}

// WithHTTPClient sets the HTTP client used for all requests.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(cfg *clientConfig) error {
		if httpClient == nil {
			return errors.New("elevenlabs: nil HTTP client")
		}
		cfg.httpClient = httpClient
		return nil
	}
}

// WithUserAgent appends product information to the SDK User-Agent.
func WithUserAgent(userAgent string) Option {
	return func(cfg *clientConfig) error {
		cfg.userAgent = strings.TrimSpace(userAgent)
		return nil
	}
}

// WithRetryPolicy opts the client into retries.
func WithRetryPolicy(policy RetryPolicy) Option {
	return func(cfg *clientConfig) error {
		if policy.MaxAttempts < 1 {
			return errors.New("elevenlabs: retry MaxAttempts must be at least 1")
		}
		if policy.BaseDelay < 0 || policy.MaxDelay < 0 {
			return errors.New("elevenlabs: retry delays cannot be negative")
		}
		if policy.MaxDelay > 0 && policy.BaseDelay > policy.MaxDelay {
			return errors.New("elevenlabs: retry BaseDelay cannot exceed MaxDelay")
		}
		cfg.retryPolicy = policy
		return nil
	}
}

// NewClient creates an ElevenLabs API client.
func NewClient(opts ...Option) (*Client, error) {
	cfg := clientConfig{
		baseURL:     DefaultBaseURL,
		httpClient:  http.DefaultClient,
		userAgent:   defaultUserAgent,
		retryPolicy: RetryPolicy{MaxAttempts: 1},
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.apiKey == "" {
		return nil, errors.New("elevenlabs: missing API key")
	}
	baseURL, err := parseBaseURL(cfg.baseURL)
	if err != nil {
		return nil, err
	}
	client := &Client{
		apiKey:      cfg.apiKey,
		baseURL:     baseURL,
		httpClient:  cfg.httpClient,
		userAgent:   userAgent(cfg.userAgent),
		retryPolicy: cfg.retryPolicy,
	}
	client.SpeechToText = &SpeechToTextService{client: client}
	client.User = &UserService{client: client}
	client.Models = &ModelsService{client: client}
	return client, nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("elevenlabs: missing base URL")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("elevenlabs: invalid base URL %q", raw)
	}
	return parsed, nil
}

func userAgent(configured string) string {
	if configured == "" || configured == defaultUserAgent {
		return defaultUserAgent
	}
	return defaultUserAgent + " " + configured
}

func (c *Client) endpoint(path string, query url.Values) string {
	u := *c.baseURL
	u.Path = strings.TrimRight(c.baseURL.Path, "/") + path
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String()
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path, query), body)
	if err != nil {
		return nil, fmt.Errorf("elevenlabs: build request: %w", err)
	}
	req.Header.Set("xi-api-key", c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)
	return req, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, replayable bool, build func(context.Context) (*http.Request, error), out any) error {
	body, err := c.do(ctx, method, path, replayable, build)
	if err != nil {
		return err
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("elevenlabs: decode response: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, replayable bool, build func(context.Context) (*http.Request, error)) ([]byte, error) {
	attempts := c.retryPolicy.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	if !replayable {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := build(ctx)
		if err != nil {
			return nil, err
		}
		res, err := c.httpClient.Do(req)
		if err != nil {
			if !c.shouldRetryConnection(err, replayable, attempt, attempts) {
				return nil, &RequestError{Method: method, URL: c.endpoint(path, nil), Err: err}
			}
			if err := sleepContext(ctx, c.retryDelay(attempt)); err != nil {
				return nil, &RequestError{Method: method, URL: c.endpoint(path, nil), Err: err}
			}
			continue
		}
		body, readErr := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if readErr != nil {
			return nil, &RequestError{Method: method, URL: req.URL.String(), Err: readErr}
		}
		if c.shouldRetryRateLimit(res, replayable, attempt, attempts) {
			if err := sleepContext(ctx, c.rateLimitRetryDelay(res.Header.Get("retry-after"), attempt)); err != nil {
				return nil, &RequestError{Method: method, URL: req.URL.String(), Err: err}
			}
			continue
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, newAPIError(res, body)
		}
		return body, nil
	}
	return nil, &RequestError{Method: method, URL: c.endpoint(path, nil), Err: errors.New("request failed")}
}

func (c *Client) shouldRetryConnection(err error, replayable bool, attempt, attempts int) bool {
	return c.retryPolicy.RetryConnectionErrors && replayable && attempt < attempts && isRetryableConnectionError(err)
}

func (c *Client) shouldRetryRateLimit(res *http.Response, replayable bool, attempt, attempts int) bool {
	return c.retryPolicy.RetryRateLimits && replayable && res.StatusCode == http.StatusTooManyRequests && attempt < attempts
}

func (c *Client) retryDelay(attempt int) time.Duration {
	delay := c.retryPolicy.BaseDelay
	if delay <= 0 {
		return 0
	}
	for i := 1; i < attempt; i++ {
		if c.retryPolicy.MaxDelay > 0 && delay >= c.retryPolicy.MaxDelay/2 {
			return c.retryPolicy.MaxDelay
		}
		delay *= 2
	}
	if c.retryPolicy.MaxDelay > 0 && delay > c.retryPolicy.MaxDelay {
		return c.retryPolicy.MaxDelay
	}
	return delay
}

func (c *Client) rateLimitRetryDelay(retryAfter string, attempt int) time.Duration {
	if delay, ok := parseRetryAfter(retryAfter, time.Now()); ok {
		return delay
	}
	return c.retryDelay(attempt)
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	if !when.After(now) {
		return 0, true
	}
	return when.Sub(now), true
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func isRetryableConnectionError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
