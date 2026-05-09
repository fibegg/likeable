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
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const projectResourceRefreshInterval = 10 * time.Second

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
			s.recordProjectProvisionFailure(ctx, userID, project, err)
		}
	}()
}

func (s *Server) provisionProject(ctx context.Context, userID, userEmail string, project *Project, prompt string) error {
	fibe, err := s.fibeClientForProject(ctx, project, userEmail)
	if err != nil {
		return err
	}
	if strings.TrimSpace(project.PlaygroundID) == "" {
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
		project.SelectedService = result.SelectedServiceName
		project.Repositories = projectRepositoriesFromGreenfield(project.ID, result)
		project.Services = projectServicesFromGreenfield(project.ID, result)
		project.Status = "launching"
		if err := s.store.ReplaceProjectResources(ctx, project.ID, project.Repositories, project.Services); err != nil {
			_ = fibe.DeleteProjectResources(ctx, project)
			return err
		}
		if err := s.store.UpdateProjectProvisioning(ctx, project.ID, userID, project.PlaygroundID, project.PlayspecID, project.PropID, project.RepoURL, project.PreviewURL, project.SelectedService, project.Status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				_ = fibe.DeleteProjectResources(ctx, project)
			}
			return err
		}
	} else if project.Status != "launching" {
		project.Status = "launching"
		if err := s.store.UpdateProjectStatus(ctx, project.ID, userID, project.Status); err != nil {
			return err
		}
	}
	if err := fibe.WaitPlaygroundReady(ctx, project.PlaygroundID); err != nil {
		return err
	}
	if err := fibe.WaitPreviewReachable(ctx, project.PreviewURL); err != nil {
		return err
	}
	project.Status = "ready"
	if err := s.store.UpdateProjectProvisioning(ctx, project.ID, userID, project.PlaygroundID, project.PlayspecID, project.PropID, project.RepoURL, project.PreviewURL, project.SelectedService, project.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_ = fibe.DeleteProjectResources(ctx, project)
		}
		return err
	}
	if strings.TrimSpace(prompt) != "" {
		if err := fibe.SendMessage(ctx, project.ConversationID, projecttext.AgentPrompt(project, prompt), nil, "queue"); err != nil {
			log.Printf("send initial prompt for project %s: %v", project.ID, err)
		}
	}
	return nil
}

func (s *Server) recordProjectProvisionFailure(ctx context.Context, userID string, project *Project, err error) {
	if retryProjectProvisionLater(project, err) {
		_ = s.store.UpdateProjectStatus(ctx, project.ID, userID, "launching")
		return
	}
	_ = s.store.UpdateProjectErrorFromError(ctx, project.ID, userID, err)
}

func retryProjectProvisionLater(project *Project, err error) bool {
	if !projectHasFibeResources(project) {
		return false
	}
	return !previewEmbeddingBlocked(err)
}

func (s *Server) recoverProjectsAsync(userID, userEmail string, projects []Project) {
	for i := range projects {
		s.recoverProjectAsync(userID, userEmail, &projects[i])
	}
}

func (s *Server) recoverProjectAsync(userID, userEmail string, project *Project) {
	if projectNeedsProvisioningRecovery(project) {
		log.Printf("recover project %s by retrying provisioning", project.ID)
		s.provisionProjectAsync(userID, userEmail, project.ID, "")
		return
	}
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
		if err := s.recoverProjectReadiness(ctx, userID, current, fibe); err != nil {
			return
		}
	}()
}

func (s *Server) recoverProjectReadiness(ctx context.Context, userID string, project *Project, fibe *fibe.Client) error {
	if _, ready, _, err := s.promoteProjectFromReachablePreview(ctx, userID, project); ready || previewEmbeddingBlocked(err) {
		return err
	}
	ready, status, err := fibe.PlaygroundReady(ctx, project.PlaygroundID)
	if err != nil {
		return err
	}
	if !ready {
		if strings.TrimSpace(project.PreviewURL) != "" {
			_ = s.store.UpdateProjectStatus(ctx, project.ID, userID, "launching")
			return fmt.Errorf("workspace is still converging: %s", status)
		}
		return fmt.Errorf("workspace is still starting: %s", status)
	}
	if strings.TrimSpace(project.PreviewURL) == "" {
		if recovered, err := fibe.GreenfieldByPlaygroundID(ctx, project.PlaygroundID); err == nil {
			mergeProjectGreenfieldResult(project, recovered)
			if err := s.store.ReplaceProjectResources(ctx, project.ID, project.Repositories, project.Services); err != nil {
				return err
			}
			if err := s.store.UpdateProjectProvisioning(ctx, project.ID, userID, project.PlaygroundID, project.PlayspecID, project.PropID, project.RepoURL, project.PreviewURL, project.SelectedService, "launching"); err != nil {
				return err
			}
		}
	}
	if _, previewReady, previewStatus, err := s.promoteProjectFromReachablePreview(ctx, userID, project); err != nil {
		return err
	} else if !previewReady {
		_ = s.store.UpdateProjectStatus(ctx, project.ID, userID, "launching")
		return fmt.Errorf("preview is still starting: %s", previewStatus)
	}
	return nil
}

func (s *Server) refreshProjectReadiness(ctx context.Context, user *User, project *Project) (*Project, error) {
	if user == nil || !projectNeedsReadinessRecovery(project) {
		return project, nil
	}
	fibe, err := s.fibeClientForProject(ctx, project, user.Email)
	if err != nil {
		return project, err
	}
	if err := s.recoverProjectReadiness(ctx, user.ID, project, fibe); err != nil {
		return project, err
	}
	updated, err := s.store.ProjectForUser(ctx, user.ID, project.ID)
	if err != nil {
		return project, err
	}
	return updated, nil
}

func (s *Server) refreshProjectResourcesIfDue(ctx context.Context, user *User, project *Project) (*Project, error) {
	return s.refreshProjectResources(ctx, user, project, false)
}

func (s *Server) refreshProjectResourcesNow(ctx context.Context, user *User, project *Project) (*Project, error) {
	return s.refreshProjectResources(ctx, user, project, true)
}

func (s *Server) refreshProjectResources(ctx context.Context, user *User, project *Project, force bool) (*Project, error) {
	if user == nil || project == nil || strings.TrimSpace(project.PlaygroundID) == "" || project.Status == "deleting" {
		return project, nil
	}
	if !force {
		key := user.ID + ":" + project.ID + ":resources"
		if last, ok := s.refreshing.Load(key); ok {
			if lastRefresh, ok := last.(time.Time); ok && time.Since(lastRefresh) < projectResourceRefreshInterval {
				return project, nil
			}
		}
		s.refreshing.Store(key, time.Now())
	}
	client, err := s.fibeClientForProject(ctx, project, user.Email)
	if err != nil {
		return project, err
	}
	result, err := client.GreenfieldByPlaygroundID(ctx, project.PlaygroundID)
	if err != nil {
		return project, err
	}
	if !greenfieldHasResourceSnapshot(result) {
		return project, nil
	}
	applyProjectGreenfieldSnapshot(project, result)
	if len(project.Repositories) > 0 || len(project.Services) > 0 {
		if err := s.store.ReplaceProjectResources(ctx, project.ID, project.Repositories, project.Services); err != nil {
			return project, err
		}
	}
	status := project.Status
	if status == "" {
		status = "ready"
	}
	if err := s.store.UpdateProjectProvisioning(ctx, project.ID, user.ID, project.PlaygroundID, project.PlayspecID, project.PropID, project.RepoURL, project.PreviewURL, project.SelectedService, status); err != nil {
		return project, err
	}
	updated, err := s.store.ProjectForUser(ctx, user.ID, project.ID)
	if err != nil {
		return project, err
	}
	return updated, nil
}

func projectNeedsReadinessRecovery(project *Project) bool {
	if project == nil {
		return false
	}
	switch project.Status {
	case "ready", "deleting":
		return false
	}
	return strings.TrimSpace(project.PlaygroundID) != ""
}

func projectNeedsProvisioningRecovery(project *Project) bool {
	if project == nil {
		return false
	}
	switch project.Status {
	case "creating", "launching":
		return strings.TrimSpace(project.PlaygroundID) == ""
	default:
		return false
	}
}

func (s *Server) promoteProjectFromReachablePreview(ctx context.Context, userID string, project *Project) (*Project, bool, string, error) {
	if project == nil || strings.TrimSpace(project.PreviewURL) == "" {
		return project, false, "starting", nil
	}
	ready, status, err := fibe.ProbePreviewURL(ctx, s.http, project.PreviewURL)
	if err != nil {
		return project, false, status, err
	}
	if !ready {
		return project, false, status, nil
	}
	if userID != "" && project.Status != "ready" {
		if err := s.store.UpdateProjectProvisioning(ctx, project.ID, userID, project.PlaygroundID, project.PlayspecID, project.PropID, project.RepoURL, project.PreviewURL, project.SelectedService, "ready"); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return project, false, status, err
		}
	}
	project.Status = "ready"
	project.ErrorMessage = ""
	return project, true, status, nil
}

func previewEmbeddingBlocked(err error) bool {
	var blocked *fibe.PreviewEmbeddingBlockedError
	return errors.As(err, &blocked)
}

func mergeProjectGreenfieldResult(project *Project, result *fibe.GreenfieldResult) {
	if project == nil || result == nil {
		return
	}
	if strings.TrimSpace(project.PlaygroundID) == "" {
		project.PlaygroundID = result.PlaygroundID
	}
	if strings.TrimSpace(project.PlayspecID) == "" {
		project.PlayspecID = result.PlayspecID
	}
	if strings.TrimSpace(project.PropID) == "" {
		project.PropID = result.PropID
	}
	if strings.TrimSpace(project.RepoURL) == "" {
		project.RepoURL = result.RepoURL
	}
	if strings.TrimSpace(project.PreviewURL) == "" {
		project.PreviewURL = result.PreviewURL
	}
	if strings.TrimSpace(project.SelectedService) == "" {
		project.SelectedService = result.SelectedServiceName
	}
	if len(project.Repositories) == 0 {
		project.Repositories = projectRepositoriesFromGreenfield(project.ID, result)
	}
	if len(project.Services) == 0 {
		project.Services = projectServicesFromGreenfield(project.ID, result)
	}
}

func greenfieldHasResourceSnapshot(result *fibe.GreenfieldResult) bool {
	return result != nil && (len(result.Repositories) > 0 || len(result.Services) > 0 || strings.TrimSpace(result.PreviewURL) != "" || strings.TrimSpace(result.PlayspecID) != "")
}

func applyProjectGreenfieldSnapshot(project *Project, result *fibe.GreenfieldResult) {
	if project == nil || result == nil {
		return
	}
	if strings.TrimSpace(result.PlaygroundID) != "" {
		project.PlaygroundID = result.PlaygroundID
	}
	if strings.TrimSpace(result.PlayspecID) != "" {
		project.PlayspecID = result.PlayspecID
	}
	if len(result.Repositories) > 0 {
		project.Repositories = projectRepositoriesFromGreenfield(project.ID, result)
	}
	if len(result.Services) > 0 {
		project.Services = projectServicesFromGreenfield(project.ID, result)
	}
	project.SelectedService = selectProjectServiceName(project.SelectedService, result.SelectedServiceName, project.Services)
	if serviceURL := projectServiceURL(project.Services, project.SelectedService); serviceURL != "" {
		project.PreviewURL = serviceURL
	} else if strings.TrimSpace(result.PreviewURL) != "" {
		project.PreviewURL = result.PreviewURL
	}
	if repository := projectRepositoryForService(project.Repositories, project.SelectedService); repository != nil {
		project.PropID = firstNonEmpty(repository.PropID, project.PropID, result.PropID)
		project.RepoURL = firstNonEmpty(repository.RepoURL, project.RepoURL, result.RepoURL)
	} else {
		project.PropID = firstNonEmpty(result.PropID, project.PropID)
		project.RepoURL = firstNonEmpty(result.RepoURL, project.RepoURL)
	}
}

func selectProjectServiceName(current, preferred string, services []ProjectService) string {
	current = strings.TrimSpace(current)
	preferred = strings.TrimSpace(preferred)
	if serviceNameExists(services, current) {
		return current
	}
	if serviceNameExists(services, preferred) {
		return preferred
	}
	for _, service := range services {
		if strings.TrimSpace(service.Name) != "" {
			return service.Name
		}
	}
	return preferred
}

func serviceNameExists(services []ProjectService, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, service := range services {
		if strings.EqualFold(service.Name, name) {
			return true
		}
	}
	return false
}

func projectServiceURL(services []ProjectService, name string) string {
	name = strings.TrimSpace(name)
	for _, service := range services {
		if strings.EqualFold(service.Name, name) && strings.TrimSpace(service.URL) != "" {
			return service.URL
		}
	}
	return ""
}

func projectRepositoryForService(repositories []ProjectRepository, serviceName string) *ProjectRepository {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return nil
	}
	for i := range repositories {
		for _, candidate := range repositories[i].ServiceNames {
			if strings.EqualFold(candidate, serviceName) {
				return &repositories[i]
			}
		}
	}
	return nil
}

func projectRepositoriesFromGreenfield(projectID string, result *fibe.GreenfieldResult) []ProjectRepository {
	if result == nil {
		return nil
	}
	out := make([]ProjectRepository, 0, len(result.Repositories))
	for _, repository := range result.Repositories {
		out = append(out, ProjectRepository{
			ID:            uuid.NewString(),
			ProjectID:     projectID,
			Role:          repository.Role,
			PropID:        repository.PropID,
			RepoURL:       repository.RepoURL,
			SourceRepoURL: repository.SourceRepoURL,
			Provider:      repository.Provider,
			ServiceNames:  append([]string(nil), repository.ServiceNames...),
		})
	}
	return out
}

func projectServicesFromGreenfield(projectID string, result *fibe.GreenfieldResult) []ProjectService {
	if result == nil {
		return nil
	}
	out := make([]ProjectService, 0, len(result.Services))
	for _, service := range result.Services {
		out = append(out, ProjectService{
			ID:           uuid.NewString(),
			ProjectID:    projectID,
			Name:         service.Name,
			URL:          service.URL,
			Type:         service.Type,
			Visibility:   service.Visibility,
			AuthRequired: service.AuthRequired,
		})
	}
	return out
}
