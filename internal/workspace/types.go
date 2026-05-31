package workspace

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/fibegg/likeable/internal/domain"
)

type Project = domain.Project
type ProjectRepository = domain.ProjectRepository
type ProjectService = domain.ProjectService

type Config struct {
	BaseURL       string
	DataDir       string
	WorkspaceRoot string
	OpenAIAPIKey  string
	OpenAIModel   string
	HTTP          *http.Client
	Project       *Project
}

type Client struct {
	baseURL       string
	dataDir       string
	workspaceRoot string
	openAIAPIKey  string
	openAIModel   string
	http          *http.Client
	project       *Project
}

type Assignment struct {
	Label     string `json:"label,omitempty"`
	AgentID   string `json:"agent_id"`
	MarqueeID string `json:"server_id"`
	Status    string `json:"status,omitempty"`
	Capacity  int    `json:"capacity,omitempty"`
}

type GreenfieldResult struct {
	PlaygroundID        string
	PlaygroundName      string
	PlaygroundStatus    string
	PlaygroundError     string
	PlayspecID          string
	PropID              string
	RepoURL             string
	PreviewURL          string
	SelectedServiceName string
	Repositories        []GreenfieldRepository
	Services            []GreenfieldService
}

type GreenfieldRepository struct {
	Role          string
	PropID        string
	RepoURL       string
	SourceRepoURL string
	Provider      string
	ServiceNames  []string
}

type GreenfieldService struct {
	Name         string
	URL          string
	Type         string
	Visibility   string
	AuthRequired bool
}

type ConversationLiveState struct {
	ConversationID    string `json:"conversationId,omitempty"`
	ConversationIDAlt string `json:"conversation_id,omitempty"`
	IsProcessing      bool   `json:"isProcessing"`
	StreamText        string `json:"streamText"`
	CurrentActivityID string `json:"currentActivityId,omitempty"`
	QueuedTurns       int    `json:"queuedTurns,omitempty"`
	StartedAt         string `json:"startedAt,omitempty"`
}

type PlatformError struct {
	Code    string
	Status  int
	Message string
	Stderr  string
}

func (e *PlatformError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{}
	if strings.TrimSpace(e.Code) != "" {
		parts = append(parts, strings.TrimSpace(e.Code))
	}
	if e.Status != 0 {
		parts = append(parts, fmt.Sprintf("status %d", e.Status))
	}
	if strings.TrimSpace(e.Message) != "" {
		parts = append(parts, strings.TrimSpace(e.Message))
	}
	if strings.TrimSpace(e.Stderr) != "" {
		parts = append(parts, strings.TrimSpace(e.Stderr))
	}
	if len(parts) == 0 {
		return "workspace error"
	}
	return "workspace: " + strings.Join(parts, ": ")
}

func (e *PlatformError) PublicProjectErrorKind() string {
	if e == nil {
		return ""
	}
	switch strings.ToUpper(strings.TrimSpace(e.Code)) {
	case "CONFIGURATION", "OPENAI_API_KEY_MISSING":
		return "configuration"
	case "TIMEOUT":
		return "timeout"
	default:
		return ""
	}
}

func NewClient(config Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	httpClient := config.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	dataDir := strings.TrimSpace(config.DataDir)
	workspaceRoot := strings.TrimSpace(config.WorkspaceRoot)
	if workspaceRoot == "" && dataDir != "" {
		workspaceRoot = dataDir + "/workspaces"
	}
	if workspaceRoot == "" {
		return nil, errors.New("workspace data directory is not configured")
	}
	model := strings.TrimSpace(config.OpenAIModel)
	if model == "" {
		model = "gpt-5-mini"
	}
	return &Client{
		baseURL:       baseURL,
		dataDir:       dataDir,
		workspaceRoot: workspaceRoot,
		openAIAPIKey:  strings.TrimSpace(config.OpenAIAPIKey),
		openAIModel:   model,
		http:          httpClient,
		project:       config.Project,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "<nil>" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
