package likeable

import (
	"context"
	"errors"
	"strings"

	"github.com/fibegg/likeable/internal/fibe"
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
	case fibe.AssignmentStatusActive, fibe.AssignmentStatusDraining:
		return nil
	case fibe.AssignmentStatusRetiring:
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
		return fibe.AssignmentStatusActive, nil
	}
	if project.Status == "archived" {
		return fibe.AssignmentStatusRetired, nil
	}
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return "", err
	}
	agentID := strings.TrimSpace(project.AgentID)
	marqueeID := strings.TrimSpace(project.MarqueeID)
	global := fibe.GlobalAssignment(cfg)
	pool, err := fibe.AssignmentPoolFromConfig(cfg)
	if err != nil {
		return "", err
	}
	if len(pool) == 0 {
		return fibe.AssignmentStatusActive, nil
	}
	if agentID == "" {
		if global.AgentID != "" {
			return fibe.AssignmentStatusActive, nil
		}
		return fibe.AssignmentStatusRetired, nil
	}
	for _, assignment := range pool {
		if strings.TrimSpace(assignment.AgentID) == agentID && strings.TrimSpace(assignment.MarqueeID) == marqueeID {
			return fibe.AssignmentStatus(assignment), nil
		}
	}
	if global.AgentID != "" && agentID == global.AgentID && (global.MarqueeID == "" || marqueeID == global.MarqueeID) {
		return fibe.AssignmentStatusActive, nil
	}
	return fibe.AssignmentStatusRetired, nil
}

func developmentBlockedMessage(err error) string {
	if errors.Is(err, errProjectRetiring) {
		return errProjectRetiring.Error()
	}
	return errProjectExportOnly.Error()
}
