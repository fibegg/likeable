package workspace

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

func IsAgentRuntimeUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	var platformErr *PlatformError
	if errors.As(err, &platformErr) {
		text = strings.ToLower(strings.Join([]string{
			platformErr.Code,
			platformErr.Message,
			platformErr.Stderr,
			err.Error(),
		}, " "))
	}
	return containsAny(text,
		"openai api key",
		"agent unavailable",
		"connection refused",
		"runtime reachable: no",
	)
}

func IsConversationMissingError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "conversation") && containsAny(text, "not found", "http 404")
}

func IsPlaygroundAlreadyStoppedError(err error) bool {
	if err == nil {
		return false
	}
	return containsAny(strings.ToLower(err.Error()), "already stopped", "not running")
}

func IsPlaygroundMissingError(err error) bool {
	if err == nil {
		return false
	}
	return containsAny(strings.ToLower(err.Error()), "workspace not found", "playground not found")
}

func IsRetryableProvisioningError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var platformErr *PlatformError
	if errors.As(err, &platformErr) {
		if platformErr.Status == http.StatusRequestTimeout ||
			(platformErr.Status == http.StatusUnprocessableEntity && strings.EqualFold(strings.TrimSpace(platformErr.Code), "INTERNAL_ERROR")) ||
			platformErr.Status == http.StatusTooManyRequests ||
			platformErr.Status == http.StatusBadGateway ||
			platformErr.Status == http.StatusServiceUnavailable ||
			platformErr.Status == http.StatusGatewayTimeout {
			return true
		}
		code := strings.ToUpper(strings.TrimSpace(platformErr.Code))
		if code == "TIMEOUT" || code == "SERVICE_UNAVAILABLE" || code == "RATE_LIMITED" {
			return true
		}
	}
	text := strings.ToLower(err.Error())
	return containsAny(text, "timeout", "temporarily unavailable", "rate limit", "too many requests")
}
