package fibe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type playgroundStatus struct {
	Status string `json:"status"`
}

type PreviewEmbeddingBlockedError struct {
	Header string
}

func (e *PreviewEmbeddingBlockedError) Error() string {
	return fmt.Sprintf("preview URL is reachable but blocks iframe embedding with %s", e.Header)
}

type PreviewTimeoutError struct {
	Status string
}

func (e *PreviewTimeoutError) Error() string {
	return fmt.Sprintf("preview URL did not become reachable: %s", e.Status)
}

func (e *PreviewTimeoutError) PublicProjectErrorKind() string {
	return "timeout"
}

type PreviewProbeResult struct {
	Ready       bool
	Displayable bool
	Maintenance bool
	Status      string
}

func (c *Client) WaitPlaygroundReady(ctx context.Context, playgroundID string) error {
	playgroundID = strings.TrimSpace(playgroundID)
	if playgroundID == "" {
		return errors.New("workspace creation did not return an id")
	}
	return c.runCLI(ctx, []string{"wait", "playground", playgroundID, "--status", "running", "--timeout", "8m", "--interval", "3s"}, nil, nil)
}

func (c *Client) PlaygroundReady(ctx context.Context, playgroundID string) (bool, string, error) {
	playgroundID = strings.TrimSpace(playgroundID)
	if playgroundID == "" {
		return false, "", errors.New("workspace creation did not return an id")
	}
	var status playgroundStatus
	if err := c.runCLI(ctx, []string{"playgrounds", "get", playgroundID}, nil, &status); err != nil {
		return false, "", err
	}
	currentStatus := strings.ToLower(strings.TrimSpace(status.Status))
	return currentStatus == "running" || currentStatus == "ready", currentStatus, nil
}

func (c *Client) WaitPreviewReachable(ctx context.Context, previewURL string) error {
	previewURL = strings.TrimSpace(previewURL)
	if previewURL == "" {
		return errors.New("workspace creation did not return a preview URL")
	}
	deadline := time.Now().Add(6 * time.Minute)
	var lastStatus string
	for time.Now().Before(deadline) {
		ready, currentStatus, err := c.PreviewReachable(ctx, previewURL)
		lastStatus = currentStatus
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	return &PreviewTimeoutError{Status: lastStatus}
}

func (c *Client) PreviewReachable(ctx context.Context, previewURL string) (bool, string, error) {
	return ProbePreviewURL(ctx, c.http, previewURL)
}

func ProbePreviewURL(ctx context.Context, client *http.Client, previewURL string) (bool, string, error) {
	result, err := ProbePreviewURLResult(ctx, client, previewURL)
	return result.Ready, result.Status, err
}

func ProbePreviewURLResult(ctx context.Context, client *http.Client, previewURL string) (PreviewProbeResult, error) {
	previewURL = strings.TrimSpace(previewURL)
	if previewURL == "" {
		return PreviewProbeResult{}, errors.New("workspace creation did not return a preview URL")
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, previewURL, nil)
	if err != nil {
		return PreviewProbeResult{}, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "likeable-preview-probe/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return PreviewProbeResult{Status: err.Error()}, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	_, _ = io.Copy(io.Discard, resp.Body)
	result := PreviewProbeResult{Status: resp.Status}
	if previewMaintenancePage(resp.StatusCode, resp.Header, body) {
		result.Displayable = true
		result.Maintenance = true
		return result, nil
	}
	if !previewStatusReady(resp.StatusCode) {
		return result, nil
	}
	if header := frameBlockingHeader(resp.Header); header != "" {
		return result, &PreviewEmbeddingBlockedError{Header: header}
	}
	result.Ready = true
	result.Displayable = true
	return result, nil
}

func previewStatusReady(status int) bool {
	if status == http.StatusNotFound {
		return false
	}
	return status >= 200 && status < 500
}

func frameBlockingHeader(headers http.Header) string {
	if value := strings.TrimSpace(headers.Get("X-Frame-Options")); value != "" {
		return "X-Frame-Options: " + value
	}
	csp := strings.TrimSpace(headers.Get("Content-Security-Policy"))
	if csp == "" {
		return ""
	}
	for _, directive := range strings.Split(csp, ";") {
		directive = strings.TrimSpace(strings.ToLower(directive))
		if directive == "frame-ancestors 'none'" || directive == "frame-ancestors 'self'" {
			return "Content-Security-Policy: frame-ancestors"
		}
	}
	return ""
}

func previewMaintenancePage(status int, headers http.Header, body []byte) bool {
	if status != http.StatusServiceUnavailable {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(headers.Get("X-Fibe-Maintenance")), "true") {
		return true
	}
	html := strings.ToLower(string(body))
	return strings.Contains(html, "<title>maintenance</title>") && strings.Contains(html, "maintenance is ongoing")
}
