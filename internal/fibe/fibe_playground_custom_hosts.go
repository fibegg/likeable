package fibe

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

func (c *Client) UpdatePlaygroundServiceCustomHosts(ctx context.Context, playgroundID string, serviceHosts map[string][]string) error {
	playgroundID = strings.TrimSpace(playgroundID)
	if playgroundID == "" {
		return errors.New("playground id is required")
	}
	services := make(map[string]map[string][]string, len(serviceHosts))
	for serviceName, hosts := range serviceHosts {
		serviceName = strings.TrimSpace(serviceName)
		if serviceName == "" {
			continue
		}
		services[serviceName] = map[string][]string{"custom_hosts": normalizeCustomHosts(hosts)}
	}
	if len(services) == 0 {
		return nil
	}
	body := map[string]any{"playground": map[string]any{"services": services}}
	return c.patchJSON(ctx, "/api/playgrounds/"+url.PathEscape(playgroundID), body)
}

func normalizeCustomHosts(hosts []string) []string {
	out := make([]string, 0, len(hosts))
	seen := map[string]bool{}
	for _, host := range hosts {
		host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), ".")
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	return out
}

func (c *Client) patchJSON(ctx context.Context, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("fibe: marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("fibe: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return platformErrorFromResponse(resp.StatusCode, resp.Status, respBody)
}

func platformErrorFromResponse(statusCode int, status string, body []byte) error {
	var payload struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"message"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)
	message := firstNonEmpty(payload.Error.Message, status)
	return &PlatformError{
		Code:    firstNonEmpty(payload.Error.Code, platformCodeUnknown),
		Status:  statusCode,
		Message: message,
		Details: payload.Error.Details,
		Stderr:  strings.TrimSpace(string(body)),
	}
}
