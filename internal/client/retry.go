package client

// Throttling policy: when Netflix says slow down, how long to wait.

import (
	"errors"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Automatic backoff on throttling (HTTP 429/503).
const (
	maxRetries       = 3
	defaultRetryBase = 500 * time.Millisecond
	maxRetryWait     = 10 * time.Second
)

func isRateLimited(err error) bool {
	status, ok := httpStatus(err)
	return ok && (status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable)
}

func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return -1
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if at, err := http.ParseTime(h); err == nil {
		if wait := time.Until(at); wait > 0 {
			return wait
		}
		return 0
	}
	return -1
}

func retryWait(err error, backoff time.Duration) time.Duration {
	var ae *APIError
	if errors.As(err, &ae) && ae.RetryAfter >= 0 {
		return capWait(ae.RetryAfter)
	}
	return capWait(backoff + time.Duration(rand.Int63n(int64(250*time.Millisecond))))
}

func capWait(d time.Duration) time.Duration {
	if d > maxRetryWait {
		return maxRetryWait
	}
	if d < 0 {
		return 0
	}
	return d
}
