package likeable

import (
	"context"
	"strings"

	"github.com/fibegg/likeable/internal/fibe"
)

func (s *Server) fibeClient(ctx context.Context) (*fibe.Client, error) {
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return nil, err
	}
	return s.fibeClientFromConfig(cfg, fibe.GlobalAssignment(cfg))
}

func (s *Server) fibeClientForProject(ctx context.Context, project *Project, email string) (*fibe.Client, error) {
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return nil, err
	}
	seed := email
	if project != nil && strings.TrimSpace(project.ID) != "" {
		seed = project.ID
	}
	assignment, err := fibe.AssignmentForProject(cfg, project, seed)
	if err != nil {
		return nil, err
	}
	return s.fibeClientFromConfig(cfg, assignment)
}

func (s *Server) fibeClientFromConfig(cfg map[string]string, assignment fibe.Assignment) (*fibe.Client, error) {
	return fibe.NewClient(fibe.Config{
		BaseURL:           strings.TrimSpace(cfg["fibe_base_url"]),
		APIKey:            strings.TrimSpace(cfg["fibe_api_key"]),
		AgentID:           assignment.AgentID,
		MarqueeID:         assignment.MarqueeID,
		TemplateVersionID: strings.TrimSpace(cfg["fibe_template_version_id"]),
		HTTP:              s.http,
	})
}
