package likeable

import (
	"context"
	"log"
	"strings"
	"time"
)

func (s *Server) startProductionProjectIfStopped(ctx context.Context, userID, projectID string) {
	project, err := s.store.ProjectForUser(ctx, userID, projectID)
	if err != nil {
		log.Printf("load production project for start %s/%s: %v", userID, projectID, err)
		return
	}
	if project.Status != "stopped" {
		return
	}
	playgroundID := strings.TrimSpace(project.PlaygroundID)
	if playgroundID == "" {
		return
	}
	user, err := s.store.UserByID(ctx, userID)
	if err != nil {
		log.Printf("load production user for start %s: %v", userID, err)
		return
	}
	fibeClient, err := s.fibeClientForProject(ctx, project, user.Email)
	if err != nil {
		log.Printf("load Fibe client for production start project=%s playground=%s: %v", project.ID, playgroundID, err)
		return
	}
	actionCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := fibeClient.StartPlayground(actionCtx, playgroundID); err != nil {
		s.observePlatformError(err)
		log.Printf("start production project playground project=%s playground=%s: %v", project.ID, playgroundID, err)
		return
	}
	if err := s.store.UpdateProjectStatus(ctx, project.ID, user.ID, "launching"); err != nil {
		log.Printf("mark production project launching project=%s playground=%s: %v", project.ID, playgroundID, err)
		return
	}
	if err := s.store.TouchProjectPlaygroundUsage(ctx, project.ID, user.ID); err != nil {
		log.Printf("touch production project usage project=%s playground=%s: %v", project.ID, playgroundID, err)
		return
	}
	s.clearPlatformBackoff()
}
