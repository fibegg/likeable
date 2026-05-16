package likeable

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	fibegateway "github.com/fibegg/likeable/internal/fibe"
)

const platformBackoff = 60 * time.Second

type platformBackoffState struct {
	mu    sync.Mutex
	until time.Time
}

func (s *Server) platformBackoffRemaining() (time.Duration, bool) {
	s.platform.mu.Lock()
	defer s.platform.mu.Unlock()
	if s.platform.until.IsZero() {
		return 0, false
	}
	remaining := time.Until(s.platform.until)
	if remaining <= 0 {
		s.platform.until = time.Time{}
		return 0, false
	}
	return remaining, true
}

func (s *Server) observePlatformError(err error) {
	if !isPlatformBackoffError(err) {
		return
	}
	s.platform.mu.Lock()
	defer s.platform.mu.Unlock()
	until := time.Now().Add(platformBackoff)
	if until.After(s.platform.until) {
		s.platform.until = until
	}
}

func (s *Server) clearPlatformBackoff() {
	s.platform.mu.Lock()
	defer s.platform.mu.Unlock()
	s.platform.until = time.Time{}
}

func isPlatformRateLimited(err error) bool {
	if err == nil {
		return false
	}
	var platformErr *fibegateway.PlatformError
	if errors.As(err, &platformErr) {
		code := strings.ToUpper(strings.TrimSpace(platformErr.Code))
		message := strings.ToLower(strings.TrimSpace(platformErr.Message + "\n" + platformErr.Stderr))
		if platformErr.Status == http.StatusTooManyRequests {
			return true
		}
		if code == "RATE_LIMITED" || code == "TOO_MANY_REQUESTS" {
			return true
		}
		if strings.Contains(message, "status 429") || strings.Contains(message, "too many requests") || strings.Contains(message, "rate limit") {
			return true
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "status 429") || strings.Contains(message, "too many requests") || strings.Contains(message, "rate limit")
}

func isPlatformBackoffError(err error) bool {
	if err == nil {
		return false
	}
	if isPlatformRateLimited(err) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var platformErr *fibegateway.PlatformError
	if errors.As(err, &platformErr) {
		code := strings.ToUpper(strings.TrimSpace(platformErr.Code))
		message := strings.ToLower(strings.TrimSpace(platformErr.Message + "\n" + platformErr.Stderr))
		if platformErr.Status == http.StatusRequestTimeout || platformErr.Status == http.StatusTooEarly || platformErr.Status == http.StatusBadGateway || platformErr.Status == http.StatusServiceUnavailable || platformErr.Status == http.StatusGatewayTimeout {
			return true
		}
		if code == "SERVICE_UNAVAILABLE" || code == "TIMEOUT" {
			return true
		}
		if containsAny(message, "context deadline exceeded", "client.timeout", "timeout awaiting headers", "signal: killed") {
			return true
		}
	}
	message := strings.ToLower(err.Error())
	return containsAny(message, "context deadline exceeded", "client.timeout", "timeout awaiting headers", "signal: killed")
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
