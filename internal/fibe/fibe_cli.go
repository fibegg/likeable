package fibe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	platformCodeUnknown          = "UNKNOWN_ERROR"
	platformCodeCLINotConfigured = "FIBE_CLI_NOT_CONFIGURED"
	platformCodeCLINotFound      = "FIBE_CLI_NOT_FOUND"
)

type PlatformError struct {
	Code    string
	Status  int
	Message string
	Details map[string]any
	Stderr  string
	Cause   error
}

func (e *PlatformError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "platform command failed"
	}
	if e.Code != "" && e.Status > 0 {
		return fmt.Sprintf("platform command failed: %s (%d): %s", e.Code, e.Status, message)
	}
	if e.Code != "" {
		return fmt.Sprintf("platform command failed: %s: %s", e.Code, message)
	}
	return "platform command failed: " + message
}

func (e *PlatformError) Unwrap() error {
	return e.Cause
}

func (e *PlatformError) PublicProjectErrorKind() string {
	switch {
	case e.Code == platformCodeCLINotConfigured || e.Code == platformCodeCLINotFound:
		return "configuration"
	case e.Status == 401 || e.Status == 403:
		return "configuration"
	default:
		return ""
	}
}

func (c *Client) runCLI(ctx context.Context, args []string, input any, out any) error {
	if strings.TrimSpace(c.cliPath) == "" {
		return &PlatformError{
			Code:    platformCodeCLINotConfigured,
			Message: "Fibe CLI path is not configured",
		}
	}
	fullArgs := append([]string{"--domain", c.cliDomain, "--api-key", c.apiKey, "--output", "json"}, args...)
	cmd := exec.CommandContext(ctx, c.cliPath, fullArgs...)
	cmd.Env = append(os.Environ(),
		"FIBE_DOMAIN="+c.cliDomain,
		"FIBE_API_KEY="+c.apiKey,
		"FIBE_OUTPUT=json",
		"NO_COLOR=1",
	)
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		cmd.Stdin = bytes.NewReader(data)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return parsePlatformError(stderr.String(), err)
	}
	if out == nil {
		return nil
	}
	data := bytes.TrimSpace(stdout.Bytes())
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

type cliErrorEnvelope struct {
	Error struct {
		Message string         `json:"message"`
		Code    string         `json:"code"`
		Status  int            `json:"status"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

func parsePlatformError(stderr string, err error) error {
	clean := sanitizeCLIError(stderr, err)
	var payload cliErrorEnvelope
	if json.Unmarshal([]byte(clean), &payload) == nil && payload.Error.Message != "" {
		return &PlatformError{
			Code:    firstNonEmpty(payload.Error.Code, platformCodeUnknown),
			Status:  payload.Error.Status,
			Message: payload.Error.Message,
			Details: payload.Error.Details,
			Stderr:  clean,
			Cause:   err,
		}
	}

	code := platformCodeUnknown
	if errors.Is(err, exec.ErrNotFound) {
		code = platformCodeCLINotFound
	}
	return &PlatformError{
		Code:    code,
		Message: clean,
		Stderr:  clean,
		Cause:   err,
	}
}

func sanitizeCLIError(stderr string, err error) string {
	message := strings.TrimSpace(stderr)
	if message == "" {
		message = err.Error()
	}
	return strings.TrimSpace(strings.ReplaceAll(message, "\x00", ""))
}
