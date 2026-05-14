package likeable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fibegg/likeable/internal/fibe"
	projecttext "github.com/fibegg/likeable/internal/project"
	"github.com/hibiken/asynq"
)

func (s *Server) deleteProjectResourcesAsync(userID, userEmail string, project *Project) {
	if s.jobs != nil {
		if err := s.enqueueProjectJob(context.Background(), taskDeleteProjectResources, projectJobPayload{UserID: userID, UserEmail: userEmail, ProjectID: project.ID}, asynq.Queue("critical"), asynq.MaxRetry(10), asynq.Timeout(20*time.Minute), asynq.Unique(30*time.Second)); err != nil {
			log.Printf("enqueue project delete %s: %v", project.ID, err)
		}
		s.enqueueProjectDeletionSweep(context.Background(), 2*time.Minute)
		return
	}
	snapshot := *project
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if snapshot.Status == "archived" {
			if err := s.deleteProjectLocally(ctx, &snapshot, userID); err != nil {
				log.Printf("delete archived local project %s: %v", snapshot.ID, err)
			}
			return
		}
		fibeClient, err := s.completeProjectResourceSnapshot(ctx, userEmail, &snapshot)
		if err != nil {
			log.Printf("delete project %s resources: %v", snapshot.ID, err)
			_ = s.store.UpdateProjectCleanupError(ctx, snapshot.ID, userID, err.Error())
			return
		}
		if projectHasFibeResources(&snapshot) {
			if err := fibeClient.DeleteProjectResources(ctx, &snapshot); err != nil {
				log.Printf("delete project %s resources: %v", snapshot.ID, err)
				_ = s.store.UpdateProjectCleanupError(ctx, snapshot.ID, userID, err.Error())
				return
			}
		} else {
			log.Printf("delete project %s resources: no remote resources found", snapshot.ID)
		}
		if err := s.store.DeleteProject(ctx, snapshot.ID, userID); err != nil {
			log.Printf("delete local project %s: %v", snapshot.ID, err)
		}
	}()
}

func (s *Server) completeProjectResourceSnapshot(ctx context.Context, userEmail string, project *Project) (*fibe.Client, error) {
	fibeClient, err := s.fibeClientForProject(ctx, project, userEmail)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(project.PlaygroundName) == "" {
		project.PlaygroundName = projecttext.SourceNameForProject(project)
	}
	if strings.TrimSpace(project.PlaygroundID) != "" {
		recovered, err := fibeClient.GreenfieldByPlaygroundID(ctx, project.PlaygroundID)
		if err != nil {
			return fibeClient, err
		}
		mergeProjectGreenfieldResult(project, recovered)
		return fibeClient, nil
	}
	recovered, err := fibeClient.FindGreenfieldBySubdomain(ctx, projecttext.PreviewSubdomain(project))
	if err != nil {
		return fibeClient, nil
	}
	mergeProjectGreenfieldResult(project, recovered)
	return fibeClient, nil
}

func (s *Server) deleteProjectLocally(ctx context.Context, project *Project, userID string) error {
	if err := s.deleteLocalProjectAttachmentDirs([]Project{*project}); err != nil {
		return err
	}
	if err := s.store.DeleteProject(ctx, project.ID, userID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("delete local project: %w", err)
	}
	return nil
}

func projectHasProvisionedResources(project *Project) bool {
	return project != nil && (project.PlaygroundID != "" || project.PlayspecID != "" || project.PropID != "" || project.RepoURL != "" || project.PreviewURL != "" || len(project.Repositories) > 0 || len(project.Services) > 0)
}
