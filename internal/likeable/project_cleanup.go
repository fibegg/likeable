package likeable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	projecttext "github.com/fibegg/likeable/internal/project"
	"github.com/fibegg/likeable/internal/workspace"
	"github.com/hibiken/asynq"
)

func (s *Server) deleteProjectResourcesAsync(userID, userEmail string, project *Project) {
	if s.jobs != nil {
		log.Printf("cleanup transition=queued project_id=%s user_id=%s source=async", project.ID, userID)
		if err := s.enqueueProjectJob(context.Background(), taskDeleteProjectResources, projectJobPayload{UserID: userID, UserEmail: userEmail, ProjectID: project.ID}, asynq.Queue(projectCleanupQueue), asynq.MaxRetry(10), asynq.Timeout(projectCleanupTaskTimeout), asynq.Unique(projectCleanupUniqueTTL)); err != nil {
			log.Printf("enqueue project delete %s: %v", project.ID, err)
		}
		s.enqueueProjectDeletionSweep(context.Background(), 2*time.Minute)
		return
	}
	snapshot := *project
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), projectCleanupTaskTimeout)
		defer cancel()
		if snapshot.Status == "archived" {
			if err := s.deleteProjectLocally(ctx, &snapshot, userID); err != nil {
				log.Printf("delete archived local project %s: %v", snapshot.ID, err)
				log.Printf("cleanup transition=failed project_id=%s user_id=%s error=%q", snapshot.ID, userID, err.Error())
			}
			return
		}
		log.Printf("cleanup transition=retrying project_id=%s user_id=%s", snapshot.ID, userID)
		workspaceClient, err := s.completeProjectResourceSnapshot(ctx, userEmail, &snapshot)
		if err != nil {
			if workspaceClient == nil || !projectHasWorkspaceResources(&snapshot) {
				log.Printf("delete project %s resources: %v", snapshot.ID, err)
				_ = s.store.UpdateProjectCleanupError(context.Background(), snapshot.ID, userID, err.Error())
				log.Printf("cleanup transition=failed project_id=%s user_id=%s error=%q", snapshot.ID, userID, err.Error())
				return
			}
			log.Printf("delete project %s resources: continuing with stored resources after snapshot error: %v", snapshot.ID, err)
		}
		if projectHasWorkspaceResources(&snapshot) {
			if err := workspaceClient.DeleteProjectResources(ctx, &snapshot); err != nil {
				log.Printf("delete project %s resources: %v", snapshot.ID, err)
				_ = s.store.UpdateProjectCleanupError(context.Background(), snapshot.ID, userID, err.Error())
				log.Printf("cleanup transition=failed project_id=%s user_id=%s error=%q", snapshot.ID, userID, err.Error())
				return
			}
		} else {
			log.Printf("delete project %s resources: no remote resources found", snapshot.ID)
		}
		if err := s.deleteProjectLocally(ctx, &snapshot, userID); err != nil {
			log.Printf("delete local project %s: %v", snapshot.ID, err)
			log.Printf("cleanup transition=failed project_id=%s user_id=%s error=%q", snapshot.ID, userID, err.Error())
			return
		}
		if err := s.finalizePendingAccountDeletionIfReady(ctx, accountDeletionPayload{UserID: userID, UserEmail: userEmail}); err != nil {
			log.Printf("finalize pending account deletion for user %s: %v", userID, err)
		}
		log.Printf("cleanup transition=succeeded project_id=%s user_id=%s", snapshot.ID, userID)
	}()
}

func (s *Server) deleteAccountAsync(userID, userEmail string) error {
	payload := accountDeletionPayload{UserID: userID, UserEmail: userEmail}
	if s.jobs != nil {
		return s.enqueueAccountDeletionJob(context.Background(), payload, asynq.Queue(projectCleanupQueue), asynq.MaxRetry(30), asynq.Timeout(projectCleanupSweepTimeout), asynq.ProcessIn(5*time.Second))
	}
	go func() {
		for attempt := 0; attempt < 120; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), projectCleanupSweepTimeout)
			err := s.finalizeAccountDeletion(ctx, payload)
			cancel()
			if err == nil {
				return
			}
			projects, projectsErr := s.store.AllProjectsForUser(context.Background(), userID)
			if projectsErr != nil {
				log.Printf("delete account %s: %v", userID, projectsErr)
				return
			}
			for i := range projects {
				if strings.TrimSpace(projects[i].CleanupLastError) != "" {
					log.Printf("delete account %s: blocked by project %s cleanup error: %s", userID, projects[i].ID, projects[i].CleanupLastError)
					return
				}
			}
			log.Printf("delete account %s: %v", userID, err)
			time.Sleep(500 * time.Millisecond)
		}
	}()
	return nil
}

func (s *Server) finalizePendingAccountDeletionIfReady(ctx context.Context, payload accountDeletionPayload) error {
	user, err := s.store.UserByID(ctx, payload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if !pendingAccountDeletion(user) {
		return nil
	}
	projects, err := s.store.AllProjectsForUser(ctx, payload.UserID)
	if err != nil {
		return err
	}
	if len(projects) > 0 {
		return nil
	}
	if normalizeEmail(payload.UserEmail) == "" {
		payload.UserEmail = user.Email
	}
	log.Printf("account deletion transition=ready user_id=%s", payload.UserID)
	return s.finalizeAccountDeletion(ctx, payload)
}

func (s *Server) completeProjectResourceSnapshot(ctx context.Context, userEmail string, project *Project) (*workspace.Client, error) {
	workspaceClient, err := s.workspaceClientForProject(ctx, project, userEmail)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(project.PlaygroundName) == "" {
		project.PlaygroundName = projecttext.SourceNameForProject(project)
	}
	if projectHasDeleteReadySnapshot(project) {
		return workspaceClient, nil
	}
	if strings.TrimSpace(project.PlaygroundID) != "" {
		recovered, err := workspaceClient.GreenfieldByPlaygroundID(ctx, project.PlaygroundID)
		if err != nil {
			return workspaceClient, err
		}
		mergeProjectGreenfieldResult(project, recovered)
		return workspaceClient, nil
	}
	recovered, err := workspaceClient.FindGreenfieldBySubdomain(ctx, projecttext.PreviewSubdomain(project))
	if err != nil {
		return workspaceClient, nil
	}
	mergeProjectGreenfieldResult(project, recovered)
	return workspaceClient, nil
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

func projectHasDeleteReadySnapshot(project *Project) bool {
	return project != nil &&
		strings.TrimSpace(project.PlaygroundID) != "" &&
		strings.TrimSpace(project.PlayspecID) != "" &&
		(strings.TrimSpace(project.PropID) != "" || strings.TrimSpace(project.RepoURL) != "" || len(project.Repositories) > 0)
}
