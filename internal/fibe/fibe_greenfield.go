package fibe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
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
		if recovered, recoverErr := c.FindGreenfieldBySubdomain(ctx, projecttext.PreviewSubdomain(project)); recoverErr == nil && recovered.PlaygroundID != "" {
			return recovered, nil
		}
		return nil, err
	}
	result := parseGreenfieldStatus(status)
	if result.PlaygroundID == "" {
		return nil, errors.New("workspace creation did not return an id")
	}
	if result.PreviewURL == "" {
		if recovered, err := c.GreenfieldByPlaygroundID(ctx, result.PlaygroundID); err == nil {
			fillMissingGreenfieldFields(result, recovered)
		}
	}
	return result, nil
}

func (c *Client) FindGreenfieldBySubdomain(ctx context.Context, subdomain string) (*GreenfieldResult, error) {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return nil, errors.New("workspace subdomain is not available")
	}
	for page := 1; page <= 5; page++ {
		playgrounds, hasMore, err := c.listPlaygrounds(ctx, page, 100)
		if err != nil {
			return nil, err
		}
		for _, playground := range playgrounds {
			id := numberString(playground["id"])
			if id == "" {
				continue
			}
			result, err := c.GreenfieldByPlaygroundID(ctx, id)
			if err != nil {
				continue
			}
			if routeMatchesSubdomain(result.PreviewURL, subdomain) {
				return result, nil
			}
		}
		if !hasMore {
			break
		}
	}
	return nil, fmt.Errorf("workspace with subdomain %q was not found", subdomain)
}

func (c *Client) GreenfieldByPlaygroundID(ctx context.Context, playgroundID string) (*GreenfieldResult, error) {
	playgroundID = strings.TrimSpace(playgroundID)
	if playgroundID == "" {
		return nil, errors.New("workspace id is not available")
	}
	var debug map[string]any
	if err := c.runCLI(ctx, []string{"playgrounds", "debug", playgroundID}, nil, &debug); err != nil {
		return nil, err
	}
	result := greenfieldResultFromDebug(debug)
	if result.PlaygroundID == "" {
		result.PlaygroundID = playgroundID
	}
	if result.PlayspecID != "" {
		_ = c.hydrateGreenfieldSource(ctx, result)
	}
	return result, nil
}

func (c *Client) listPlaygrounds(ctx context.Context, page, perPage int) ([]map[string]any, bool, error) {
	var raw map[string]any
	if err := c.runCLI(ctx, []string{"playgrounds", "list", "--page", strconv.Itoa(page), "--per-page", strconv.Itoa(perPage)}, nil, &raw); err != nil {
		return nil, false, err
	}
	items := objectSlice(firstAny(raw["Data"], raw["data"], raw["items"], raw["playgrounds"]))
	meta := anyMap(firstAny(raw["Meta"], raw["meta"]))
	hasMore := false
	if totalPages := numberInt(firstAny(meta["total_pages"], meta["totalPages"])); totalPages > page {
		hasMore = true
	} else if nextPage := numberInt(firstAny(meta["next_page"], meta["nextPage"])); nextPage > page {
		hasMore = true
	} else if len(items) == perPage {
		hasMore = true
	}
	return items, hasMore, nil
}

func greenfieldResultFromDebug(debug map[string]any) *GreenfieldResult {
	result := &GreenfieldResult{}
	diagnostics := anyMap(firstAny(debug["diagnostics"], debug["Diagnostics"]))
	playground := anyMap(firstAny(diagnostics["playground"], debug["playground"]))
	result.PlaygroundID = numberString(playground["id"])
	result.PlayspecID = numberString(firstAny(playground["playspec_id"], playground["playspecID"]))

	for _, route := range objectSlice(firstAny(diagnostics["routes"], debug["routes"])) {
		if result.PreviewURL == "" {
			result.PreviewURL = routePreviewURL(route)
		}
		if result.PreviewURL != "" {
			break
		}
	}
	return result
}

func (c *Client) hydrateGreenfieldSource(ctx context.Context, result *GreenfieldResult) error {
	if result == nil || result.PlayspecID == "" {
		return nil
	}
	var playspec map[string]any
	if err := c.runCLI(ctx, []string{"playspecs", "get", result.PlayspecID}, nil, &playspec); err != nil {
		return err
	}
	for _, service := range objectSlice(playspec["services"]) {
		if result.PropID == "" {
			result.PropID = numberString(firstAny(service["prop_id"], service["propID"]))
		}
		if result.RepoURL == "" {
			result.RepoURL = firstNonEmpty(fmt.Sprint(service["repo_url"]), fmt.Sprint(service["repository_url"]), fmt.Sprint(service["clone_url"]), fmt.Sprint(service["html_url"]))
		}
		if result.PropID != "" && result.RepoURL != "" {
			return nil
		}
	}
	return nil
}

func fillMissingGreenfieldFields(result, recovered *GreenfieldResult) {
	if result == nil || recovered == nil {
		return
	}
	if result.PlaygroundID == "" {
		result.PlaygroundID = recovered.PlaygroundID
	}
	if result.PlayspecID == "" {
		result.PlayspecID = recovered.PlayspecID
	}
	if result.PropID == "" {
		result.PropID = recovered.PropID
	}
	if result.RepoURL == "" {
		result.RepoURL = recovered.RepoURL
	}
	if result.PreviewURL == "" {
		result.PreviewURL = recovered.PreviewURL
	}
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

func routePreviewURL(route map[string]any) string {
	rawURL := firstNonEmpty(
		fmt.Sprint(route["url"]),
		fmt.Sprint(route["preview_url"]),
	)
	if rawURL != "" {
		if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
			return rawURL
		}
		return "https://" + strings.TrimLeft(rawURL, "/")
	}
	host := firstNonEmpty(
		fmt.Sprint(route["traefik_host"]),
		firstStringFromSlice(route["traefik_hosts"]),
		firstStringFromSlice(route["expected_hosts"]),
	)
	if host == "" {
		return ""
	}
	scheme := "https"
	if strings.Contains(host, ".test") || strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1") {
		scheme = "http"
	}
	return scheme + "://" + host
}

func routeMatchesSubdomain(rawURL, subdomain string) bool {
	rawURL = strings.TrimSpace(rawURL)
	subdomain = strings.TrimSpace(subdomain)
	if rawURL == "" || subdomain == "" {
		return false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == subdomain || strings.HasPrefix(host, subdomain+".")
}

func firstAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func anyMap(raw any) map[string]any {
	if value, ok := raw.(map[string]any); ok {
		return value
	}
	return map[string]any{}
}

func objectSlice(raw any) []map[string]any {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out
}

func firstStringFromSlice(raw any) string {
	values, ok := raw.([]any)
	if !ok {
		return ""
	}
	for _, value := range values {
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func numberString(raw any) string {
	switch value := raw.(type) {
	case int:
		if value > 0 {
			return strconv.Itoa(value)
		}
	case int64:
		if value > 0 {
			return strconv.FormatInt(value, 10)
		}
	case float64:
		if value > 0 {
			return strconv.FormatInt(int64(value), 10)
		}
	case json.Number:
		if text := value.String(); text != "" && text != "0" {
			return text
		}
	default:
		text := firstNonEmpty(fmt.Sprint(raw))
		if text != "" && text != "0" {
			return text
		}
	}
	return ""
}

func numberInt(raw any) int {
	text := numberString(raw)
	if text == "" {
		return 0
	}
	value, _ := strconv.Atoi(text)
	return value
}
