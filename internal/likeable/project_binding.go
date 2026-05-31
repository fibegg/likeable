package likeable

import (
	"context"
	"errors"
	"strings"

	"github.com/fibegg/likeable/internal/workspace"
)

var (
	errProjectExportOnly = errors.New("project is archived; export it or create a new project")
	errProjectRetiring   = errors.New("project is being archived; export it after archival finishes")
)

func (s *Server) ensureProjectDevelopmentAllowed(ctx context.Context, user *User, project *Project) error {
	status, err := s.projectBindingStatus(ctx, project)
	if err != nil {
		return err
	}
	if project == nil {
		return nil
	}
	if project.Status == "archived" {
		return errProjectExportOnly
	}
	switch status {
	case workspace.AssignmentStatusActive, workspace.AssignmentStatusDraining:
		return nil
	case workspace.AssignmentStatusRetiring:
		return errProjectRetiring
	default:
		if user != nil {
			_ = s.markProjectArchived(ctx, user.ID, project)
		}
		return errProjectExportOnly
	}
}

func (s *Server) projectIsExportOnly(ctx context.Context, user *User, project *Project) (bool, error) {
	if project == nil {
		return false, nil
	}
	err := s.ensureProjectDevelopmentAllowed(ctx, user, project)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, errProjectExportOnly) || errors.Is(err, errProjectRetiring) {
		return true, nil
	}
	return false, err
}

func (s *Server) markProjectArchived(ctx context.Context, userID string, project *Project) error {
	if project == nil || project.Status == "archived" || project.Status == "deleting" {
		return nil
	}
	if err := s.store.UpdateProjectStatus(ctx, project.ID, userID, "archived"); err != nil {
		return err
	}
	project.Status = "archived"
	return nil
}

func (s *Server) projectBindingStatus(ctx context.Context, project *Project) (string, error) {
	if project == nil {
		return workspace.AssignmentStatusActive, nil
	}
	if project.Status == "archived" {
		return workspace.AssignmentStatusRetired, nil
	}
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return "", err
	}
	agentID := strings.TrimSpace(project.AgentID)
	marqueeID := strings.TrimSpace(project.MarqueeID)
	global := workspace.GlobalAssignment(cfg)
	pool, err := workspace.AssignmentPoolFromConfig(cfg)
	if err != nil {
		return "", err
	}
	if len(pool) == 0 {
		return workspace.AssignmentStatusActive, nil
	}
	if agentID == "" {
		if global.AgentID != "" {
			return workspace.AssignmentStatusActive, nil
		}
		return workspace.AssignmentStatusRetired, nil
	}
	for _, assignment := range pool {
		if strings.TrimSpace(assignment.AgentID) == agentID && strings.TrimSpace(assignment.MarqueeID) == marqueeID {
			return workspace.AssignmentStatus(assignment), nil
		}
	}
	if global.AgentID != "" && agentID == global.AgentID && (global.MarqueeID == "" || marqueeID == global.MarqueeID) {
		return workspace.AssignmentStatusActive, nil
	}
	return workspace.AssignmentStatusRetired, nil
}

func developmentBlockedMessage(err error) string {
	if errors.Is(err, errProjectRetiring) {
		return errProjectRetiring.Error()
	}
	return errProjectExportOnly.Error()
}
