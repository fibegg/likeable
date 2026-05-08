package fibe

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	projecttext "github.com/fibegg/likeable/internal/project"
)

type GreenfieldResult struct {
	PlaygroundID string
	PlayspecID   string
	PropID       string
	RepoURL      string
	PreviewURL   string
}

func (c *Client) CreateConversation(ctx context.Context, conversationID, title string) error {
	var out map[string]any
	args := []string{"agents", "create-conversation", c.agentID, "--conversation-id", conversationID}
	if strings.TrimSpace(title) != "" {
		args = append(args, "--title", title)
	}
	return c.runCLI(ctx, args, nil, &out)
}

func (c *Client) CreateGreenfield(ctx context.Context, project *Project) (*GreenfieldResult, error) {
	name := projecttext.SourceName(project.Title)
	args := []string{"greenfield", "--name", name, "--git-provider", "gitea", "--private", "--wait-timeout", "10m"}
	if c.marqueeID != "" {
		args = append(args, "--marquee-id", c.marqueeID)
	}
	if c.templateVersionID != "" {
		args = append(args, "--template-version-id", c.templateVersionID)
	}
	for key, value := range greenfieldVariables(project) {
		args = append(args, "--var", key+"="+value)
	}
	var status map[string]any
	if err := c.runCLI(ctx, args, nil, &status); err != nil {
		return nil, err
	}
	result := parseGreenfieldStatus(status)
	if result.PlaygroundID == "" {
		return nil, errors.New("workspace creation did not return an id")
	}
	return result, nil
}

func parseGreenfieldStatus(status map[string]any) *GreenfieldResult {
	result := &GreenfieldResult{}
	if pg, ok := status["playground"].(map[string]any); ok {
		result.PlaygroundID = firstNonEmpty(fmt.Sprint(pg["id"]))
	}
	if playspec, ok := status["playspec"].(map[string]any); ok {
		result.PlayspecID = firstNonEmpty(fmt.Sprint(playspec["id"]))
	}
	if prop, ok := status["prop"].(map[string]any); ok {
		result.PropID = firstNonEmpty(fmt.Sprint(prop["id"]))
	}
	if result.PropID == "" {
		if props, ok := status["props"].([]any); ok && len(props) > 0 {
			if prop, ok := props[0].(map[string]any); ok {
				result.PropID = firstNonEmpty(fmt.Sprint(prop["id"]))
			}
		}
	}
	if repo, ok := status["repo"].(map[string]any); ok {
		result.RepoURL = firstNonEmpty(fmt.Sprint(repo["repository_url"]), fmt.Sprint(repo["clone_url"]), fmt.Sprint(repo["html_url"]))
	}
	result.PreviewURL = selectPreviewURL(status["service_urls"])
	return result
}

func selectPreviewURL(raw any) string {
	urls, ok := raw.([]any)
	if !ok {
		return ""
	}
	bestURL := ""
	bestScore := -1 << 30
	for _, item := range urls {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rawURL := firstNonEmpty(fmt.Sprint(entry["url"]))
		if rawURL == "" {
			continue
		}
		score := serviceURLScore(entry, rawURL)
		if score > bestScore {
			bestScore = score
			bestURL = rawURL
		}
	}
	return bestURL
}

func serviceURLScore(entry map[string]any, rawURL string) int {
	name := strings.ToLower(strings.TrimSpace(fmt.Sprint(entry["name"])))
	serviceType := strings.ToLower(strings.TrimSpace(fmt.Sprint(entry["type"])))
	visibility := strings.ToLower(strings.TrimSpace(fmt.Sprint(entry["visibility"])))
	score := 0
	if visibility == "external" || visibility == "public" {
		score += 10
	}
	if serviceType == "dynamic" {
		score += 30
	}
	if name == "app" || name == "web" || name == "frontend" {
		score += 100
	}
	if strings.Contains(name, "ws") || strings.Contains(name, "websocket") || strings.Contains(name, "cable") {
		score -= 100
	}
	if parsed, err := url.Parse(rawURL); err == nil {
		host := strings.ToLower(parsed.Hostname())
		if strings.HasPrefix(host, "ws-") || strings.Contains(host, ".ws-") {
			score -= 80
		}
	}
	return score
}

func greenfieldVariables(project *Project) map[string]string {
	subdomain := projecttext.PreviewSubdomain(project)
	return map[string]string{
		"subdomain":    subdomain,
		"ws_subdomain": "ws-" + subdomain,
	}
}
