package likeable

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fibegg/likeable/internal/fibe"
	projecttext "github.com/fibegg/likeable/internal/project"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func (s *Server) ensureDefaultProject(ctx context.Context, user *User) {
	if user == nil {
		return
	}
	projects, err := s.store.ProjectsForUser(ctx, user.ID)
	if err != nil {
		log.Printf("load projects for starter: %v", err)
		return
	}
	if len(projects) > 0 || s.projectCapForUser(ctx, user) <= 0 {
		return
	}
	project, err := s.createProjectRecord(ctx, user, "New playground")
	if err != nil {
		log.Printf("create starter project: %v", err)
		return
	}
	s.provisionProjectAsync(user.ID, user.Email, project.ID, "")
}

func (s *Server) createProjectRecord(ctx context.Context, user *User, title string) (*Project, error) {
	projectID := uuid.NewString()
	cfg, err := s.store.ConfigMap(ctx)
	if err != nil {
		return nil, err
	}
	assignment, err := fibe.AssignmentForNewProject(cfg, user.Email)
	if err != nil {
		return nil, err
	}
	project := &Project{
		ID:             projectID,
		UserID:         user.ID,
		Title:          title,
		ConversationID: "likeable-" + strings.ReplaceAll(projectID, "-", ""),
		AgentID:        assignment.AgentID,
		MarqueeID:      assignment.MarqueeID,
		Status:         "creating",
	}
	if err := s.store.CreateProject(ctx, project); err != nil {
		return nil, err
	}
	return project, nil
}

func (s *Server) provisionProjectAsync(userID, userEmail, projectID, prompt string) {
	if s.jobs != nil {
		if err := s.enqueueProjectJob(context.Background(), taskProvisionProject, projectJobPayload{UserID: userID, UserEmail: userEmail, ProjectID: projectID, Prompt: prompt}, asynq.Queue("critical"), asynq.MaxRetry(6), asynq.Timeout(15*time.Minute), asynq.Unique(30*time.Second)); err != nil {
			log.Printf("enqueue project provisioning %s: %v", projectID, err)
		}
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
		defer cancel()
		project, err := s.store.ProjectForUser(ctx, userID, projectID)
		if err != nil {
			log.Printf("load project for provisioning: %v", err)
			return
		}
		if err := s.provisionProject(ctx, userID, userEmail, project, prompt); err != nil {
			log.Printf("provision project %s: %v", project.ID, err)
			_ = s.store.UpdateProjectError(ctx, project.ID, userID, err.Error())
		}
	}()
}

func (s *Server) provisionProject(ctx context.Context, userID, userEmail string, project *Project, prompt string) error {
	fibe, err := s.fibeClientForProject(ctx, project, userEmail)
	if err != nil {
		return err
	}
	if err := fibe.CreateConversation(ctx, project.ConversationID, project.Title); err != nil {
		return err
	}
	result, err := fibe.CreateGreenfield(ctx, project)
	if err != nil {
		return err
	}
	project.PlaygroundID = result.PlaygroundID
	project.RepoURL = result.RepoURL
	project.PreviewURL = result.PreviewURL
	project.PlayspecID = result.PlayspecID
	project.PropID = result.PropID
	project.Status = "launching"
	if err := s.store.UpdateProjectProvisioning(ctx, project.ID, userID, project.PlaygroundID, project.PlayspecID, project.PropID, project.RepoURL, project.PreviewURL, project.Status); err != nil {
		return err
	}
	if err := fibe.WaitPlaygroundReady(ctx, project.PlaygroundID); err != nil {
		return err
	}
	reachable, probeStatus, err := fibe.PreviewReachable(ctx, project.PreviewURL)
	if err != nil {
		if previewEmbeddingBlocked(err) {
			return err
		}
		log.Printf("preview probe for project %s failed before readiness; continuing with running playground: %v", project.ID, err)
	} else if !reachable {
		log.Printf("preview probe for project %s is not reachable before readiness; continuing with running playground: %s", project.ID, probeStatus)
	}
	project.Status = "ready"
	if err := s.store.UpdateProjectProvisioning(ctx, project.ID, userID, project.PlaygroundID, project.PlayspecID, project.PropID, project.RepoURL, project.PreviewURL, project.Status); err != nil {
		return err
	}
	if strings.TrimSpace(prompt) != "" {
		if err := fibe.SendMessage(ctx, project.ConversationID, projecttext.AgentPrompt(project, prompt), nil, "queue"); err != nil {
			log.Printf("send initial prompt for project %s: %v", project.ID, err)
		}
	}
	return nil
}

func (s *Server) recoverProjectsAsync(userID, userEmail string, projects []Project) {
	for i := range projects {
		s.recoverProjectAsync(userID, userEmail, &projects[i])
	}
}

func (s *Server) recoverProjectAsync(userID, userEmail string, project *Project) {
	if !projectNeedsReadinessRecovery(project) {
		return
	}
	key := userID + ":" + project.ID
	if _, loaded := s.recovering.LoadOrStore(key, true); loaded {
		return
	}
	if s.jobs != nil {
		if err := s.enqueueProjectJob(context.Background(), taskRecoverProject, projectJobPayload{UserID: userID, UserEmail: userEmail, ProjectID: project.ID}, asynq.Queue("default"), asynq.MaxRetry(3), asynq.Timeout(90*time.Second), asynq.Unique(45*time.Second)); err != nil {
			log.Printf("enqueue project recovery %s: %v", project.ID, err)
		}
		s.recovering.Delete(key)
		return
	}
	go func() {
		defer s.recovering.Delete(key)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		current, err := s.store.ProjectForUser(ctx, userID, project.ID)
		if err != nil || !projectNeedsReadinessRecovery(current) {
			return
		}
		fibe, err := s.fibeClientForProject(ctx, current, userEmail)
		if err != nil {
			return
		}
		ready, status, err := fibe.PlaygroundReady(ctx, current.PlaygroundID)
		if err != nil || !ready {
			if err == nil && (status == "error" || status == "failed") {
				_ = s.store.UpdateProjectError(ctx, current.ID, userID, fmt.Sprintf("workspace is %s", status))
			}
			return
		}
		reachable, _, err := fibe.PreviewReachable(ctx, current.PreviewURL)
		if err != nil {
			return
		}
		if !reachable {
			log.Printf("preview probe for project %s is not reachable during recovery; marking ready from running playground", current.ID)
		}
		_ = s.store.UpdateProjectProvisioning(ctx, current.ID, userID, current.PlaygroundID, current.PlayspecID, current.PropID, current.RepoURL, current.PreviewURL, "ready")
	}()
}

func projectNeedsReadinessRecovery(project *Project) bool {
	if project == nil {
		return false
	}
	switch project.Status {
	case "ready", "deleting":
		return false
	}
	return strings.TrimSpace(project.PlaygroundID) != "" && strings.TrimSpace(project.PreviewURL) != ""
}

func previewEmbeddingBlocked(err error) bool {
	return err != nil && strings.Contains(err.Error(), "blocks iframe embedding")
}
