package fibe

import (
	"errors"
	"net/http"
	"strings"

	sdkfibe "github.com/fibegg/sdk/fibe"
)

type Client struct {
	baseURL           string
	apiKey            string
	agentID           string
	marqueeID         string
	templateVersionID string
	runtimeChatURL    string
	http              *http.Client
	sdk               *sdkfibe.Client
}

type Assignment struct {
	Label     string `json:"label,omitempty"`
	AgentID   string `json:"agent_id"`
	MarqueeID string `json:"server_id"`
	Status    string `json:"status,omitempty"`
	Capacity  int    `json:"capacity,omitempty"`
}

type Config struct {
	BaseURL           string
	APIKey            string
	AgentID           string
	MarqueeID         string
	TemplateVersionID string
	HTTP              *http.Client
}

func NewClient(config Config) (*Client, error) {
	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		return nil, errors.New("platform base URL is not configured")
	}
	baseURL = normalizeFibeBaseURL(baseURL)
	apiKey := strings.TrimSpace(config.APIKey)
	agentID := strings.TrimSpace(config.AgentID)
	if apiKey == "" || agentID == "" {
		return nil, errors.New("platform API key or agent ID is not configured")
	}
	httpClient := config.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	sdkClient := sdkfibe.NewClient(
		sdkfibe.WithBaseURL(baseURL),
		sdkfibe.WithAPIKey(apiKey),
		sdkfibe.WithHTTPClient(httpClient),
		sdkfibe.WithDisableAutoConfig(),
		sdkfibe.WithMaxRetries(0),
	)
	return &Client{
		baseURL:           strings.TrimRight(baseURL, "/"),
		apiKey:            apiKey,
		agentID:           agentID,
		marqueeID:         strings.TrimSpace(config.MarqueeID),
		templateVersionID: strings.TrimSpace(config.TemplateVersionID),
		http:              httpClient,
		sdk:               sdkClient,
	}, nil
}

func (c *Client) AgentID() string { return c.agentID }

func (c *Client) MarqueeID() string { return c.marqueeID }

func (c *Client) BaseURL() string { return c.baseURL }

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && value != "<nil>" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeFibeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if strings.HasPrefix(baseURL, "http://") || strings.HasPrefix(baseURL, "https://") {
		return baseURL
	}
	if strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1") || strings.Contains(baseURL, ".test") {
		return "http://" + baseURL
	}
	return "https://" + baseURL
}
