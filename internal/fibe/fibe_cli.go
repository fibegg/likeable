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

func (c *Client) runCLI(ctx context.Context, args []string, input any, out any) error {
	if strings.TrimSpace(c.cliPath) == "" {
		return errors.New("Fibe CLI path is not configured")
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
		return fmt.Errorf("platform command failed: %s", sanitizeCLIError(stderr.String(), err))
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

func sanitizeCLIError(stderr string, err error) string {
	message := strings.TrimSpace(stderr)
	if message == "" {
		message = err.Error()
	}
	return strings.TrimSpace(strings.ReplaceAll(message, "\x00", ""))
}
