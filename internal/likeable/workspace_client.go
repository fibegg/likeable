package likeable

import (
	"context"
	"os"
	"strings"

	"github.com/fibegg/likeable/internal/workspace"
)

func (s *Server) workspaceClient(ctx context.Context) (*workspace.Client, error) {
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return nil, err
	}
	return s.workspaceClientFromConfig(cfg, nil)
}

func (s *Server) workspaceClientForProject(ctx context.Context, project *Project, email string) (*workspace.Client, error) {
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return nil, err
	}
	return s.workspaceClientFromConfig(cfg, project)
}

func (s *Server) workspaceClientFromConfig(cfg map[string]string, project *Project) (*workspace.Client, error) {
	root := firstNonEmptyString(cfg["workspace_root"], os.Getenv("LIKEABLE_WORKSPACE_ROOT"))
	return workspace.NewClient(workspace.Config{
		BaseURL:       s.config.BaseURL,
		DataDir:       s.store.DataDir(),
		WorkspaceRoot: root,
		OpenAIAPIKey:  firstNonEmptyString(cfg["openai_api_key"], os.Getenv("OPENAI_API_KEY")),
		OpenAIModel:   firstNonEmptyString(cfg["openai_model"], os.Getenv("OPENAI_MODEL")),
		HTTP:          s.http,
		Project:       project,
	})
}

func workspaceProviderConfigured(cfg map[string]string) bool {
	return strings.TrimSpace(firstNonEmptyString(cfg["openai_api_key"], os.Getenv("OPENAI_API_KEY"))) != ""
}
