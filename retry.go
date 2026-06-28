package elevenlabs

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultRetryMaxAttempts = 3
	defaultRetryBaseDelay   = 200 * time.Millisecond
	defaultRetryMaxDelay    = 2 * time.Second
)

var defaultRetryStatusCodes = []int{
	http.StatusTooManyRequests,
	http.StatusInternalServerError,
	http.StatusBadGateway,
	http.StatusServiceUnavailable,
	http.StatusGatewayTimeout,
}

// RetryConfig configures automatic retries for replayable requests.
//
// MaxAttempts is the total number of attempts, including the initial request.
// Values less than or equal to 1 disable retries.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	StatusCodes []int
}

type retryConfig struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
	statusCodes map[int]bool
}

// DefaultRetryConfig returns the default retry policy.
func DefaultRetryConfig() RetryConfig {
	statusCodes := make([]int, len(defaultRetryStatusCodes))
	copy(statusCodes, defaultRetryStatusCodes)

	return RetryConfig{
		MaxAttempts: defaultRetryMaxAttempts,
		BaseDelay:   defaultRetryBaseDelay,
		MaxDelay:    defaultRetryMaxDelay,
		StatusCodes: statusCodes,
	}
}

// WithRetryConfig overrides the retry policy used for replayable requests.
func WithRetryConfig(cfg RetryConfig) ClientOption {
	return func(c *Client) {
		c.retryConfig = normalizeRetryConfig(cfg)
	}
}

// WithoutRetries disables automatic retries.
func WithoutRetries() ClientOption {
	return func(c *Client) {
		c.retryConfig = normalizeRetryConfig(RetryConfig{MaxAttempts: 1})
	}
}

func defaultRetryConfig() retryConfig {
	return normalizeRetryConfig(DefaultRetryConfig())
}

func normalizeRetryConfig(cfg RetryConfig) retryConfig {
	statusCodes := cfg.StatusCodes
	if len(statusCodes) == 0 {
		statusCodes = defaultRetryStatusCodes
	}

	retryableStatuses := make(map[int]bool, len(statusCodes))
	for _, statusCode := range statusCodes {
		retryableStatuses[statusCode] = true
	}

	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 1 {
		maxAttempts = 1
	}

	baseDelay := cfg.BaseDelay
	if baseDelay <= 0 {
		baseDelay = defaultRetryBaseDelay
	}

	maxDelay := cfg.MaxDelay
	if maxDelay <= 0 {
		maxDelay = defaultRetryMaxDelay
	}
	if maxDelay < baseDelay {
		maxDelay = baseDelay
	}

	return retryConfig{
		maxAttempts: maxAttempts,
		baseDelay:   baseDelay,
		maxDelay:    maxDelay,
		statusCodes: retryableStatuses,
	}
}

func (c retryConfig) retryableStatus(statusCode int) bool {
	return c.statusCodes[statusCode]
}

func (c retryConfig) retryDelay(attempt int, retryAfter string) time.Duration {
	if delay, ok := parseRetryAfter(retryAfter); ok {
		return delay
	}
	return c.backoffDelay(attempt)
}

func (c retryConfig) backoffDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	delay := c.baseDelay
	for i := 1; i < attempt; i++ {
		if delay >= c.maxDelay/2 {
			delay = c.maxDelay
			break
		}
		delay *= 2
	}
	if delay > c.maxDelay {
		delay = c.maxDelay
	}

	return jitterDelay(delay)
}

func parseRetryAfter(value string) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}

	seconds, err := strconv.Atoi(value)
	if err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}

	retryAt, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}

	delay := time.Until(retryAt)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

func jitterDelay(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}

	half := delay / 2
	if half <= 0 {
		return delay
	}

	return half + time.Duration(rand.Int63n(int64(half)+1))
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
