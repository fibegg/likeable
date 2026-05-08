package likeable

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	projecttext "github.com/fibegg/likeable/internal/project"
	"github.com/hibiken/asynq"
)

const (
	taskProvisionProject       = "likeable:project:provision"
	taskRecoverProject         = "likeable:project:recover"
	taskDeleteProjectResources = "likeable:project:delete_resources"
	taskProjectDeletionSweep   = "likeable:project:deletion_sweep"
	taskArchiveDeleteProject   = "likeable:project:archive_delete"
	taskSendEmail              = "likeable:email:send"
	taskProjectQuotaSweep      = "likeable:project_quota:sweep"
)

type JobSystem struct {
	client *asynq.Client
	server *asynq.Server
	mux    *asynq.ServeMux
}

type projectJobPayload struct {
	UserID    string `json:"user_id"`
	UserEmail string `json:"user_email"`
	ProjectID string `json:"project_id"`
	Prompt    string `json:"prompt,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type emailJobPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func newJobSystem(redisOpt asynq.RedisClientOpt, s *Server) *JobSystem {
	mux := asynq.NewServeMux()
	mux.HandleFunc(taskProvisionProject, s.handleProvisionProjectTask)
	mux.HandleFunc(taskRecoverProject, s.handleRecoverProjectTask)
	mux.HandleFunc(taskDeleteProjectResources, s.handleDeleteProjectResourcesTask)
	mux.HandleFunc(taskProjectDeletionSweep, s.handleProjectDeletionSweepTask)
	mux.HandleFunc(taskArchiveDeleteProject, s.handleArchiveDeleteProjectTask)
	mux.HandleFunc(taskSendEmail, s.handleSendEmailTask)
	mux.HandleFunc(taskProjectQuotaSweep, s.handleProjectQuotaSweepTask)
	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:     4,
		ShutdownTimeout: 20 * time.Second,
		Queues: map[string]int{
			"critical": 8,
			"default":  4,
			"low":      1,
		},
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			retried, _ := asynq.GetRetryCount(ctx)
			maxRetry, _ := asynq.GetMaxRetry(ctx)
			log.Printf("job %s failed retry=%d/%d: %v", task.Type(), retried, maxRetry, err)
		}),
	})
	return &JobSystem{client: asynq.NewClient(redisOpt), server: server, mux: mux}
}

func newJobClient(redisOpt asynq.RedisClientOpt) *JobSystem {
	return &JobSystem{client: asynq.NewClient(redisOpt)}
}

func (j *JobSystem) Start() {
	if j == nil || j.server == nil {
		return
	}
	go func() {
		if err := j.server.Run(j.mux); err != nil {
			log.Printf("asynq worker stopped: %v", err)
		}
	}()
}

func (j *JobSystem) Close() {
	if j == nil {
		return
	}
	if j.server != nil {
		j.server.Shutdown()
	}
	if j.client != nil {
		_ = j.client.Close()
	}
}

func (j *JobSystem) Run(ctx context.Context) error {
	if j == nil || j.server == nil {
		return errors.New("job worker is not configured")
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- j.server.Run(j.mux)
	}()
	select {
	case <-ctx.Done():
		j.server.Shutdown()
		err := <-errCh
		if err != nil {
			log.Printf("asynq worker stopped during shutdown: %v", err)
		}
		return nil
	case err := <-errCh:
		return err
	}
}

func (s *Server) enqueueProjectJob(ctx context.Context, taskType string, payload projectJobPayload, opts ...asynq.Option) error {
	if s.jobs == nil {
		return errors.New("job system is not configured")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.jobs.client.EnqueueContext(ctx, asynq.NewTask(taskType, data), opts...)
	if errors.Is(err, asynq.ErrDuplicateTask) {
		return nil
	}
	return err
}

func (s *Server) enqueueEmailJob(ctx context.Context, payload emailJobPayload) error {
	if s.jobs == nil {
		return errors.New("job system is not configured")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.jobs.client.EnqueueContext(ctx, asynq.NewTask(taskSendEmail, data), asynq.Queue("low"), asynq.MaxRetry(8), asynq.Timeout(30*time.Second))
	if errors.Is(err, asynq.ErrDuplicateTask) {
		return nil
	}
	return err
}

func (s *Server) enqueueProjectQuotaSweep(ctx context.Context, delay time.Duration) {
	if s.jobs == nil {
		return
	}
	opts := []asynq.Option{asynq.Queue("low"), asynq.MaxRetry(2), asynq.Timeout(15 * time.Minute), asynq.Unique(50 * time.Minute)}
	if delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
	}
	_, err := s.jobs.client.EnqueueContext(ctx, asynq.NewTask(taskProjectQuotaSweep, nil), opts...)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		log.Printf("enqueue project quota sweep: %v", err)
	}
}

func (s *Server) enqueueProjectDeletionSweep(ctx context.Context, delay time.Duration) {
	if s.jobs == nil {
		return
	}
	opts := []asynq.Option{asynq.Queue("critical"), asynq.MaxRetry(2), asynq.Timeout(15 * time.Minute), asynq.Unique(10 * time.Minute)}
	if delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
	}
	_, err := s.jobs.client.EnqueueContext(ctx, asynq.NewTask(taskProjectDeletionSweep, nil), opts...)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		log.Printf("enqueue project deletion sweep: %v", err)
	}
}

func (s *Server) startRecurringJobs(ctx context.Context) {
	if s.jobs == nil {
		return
	}
	s.enqueueProjectQuotaSweep(ctx, 0)
	s.enqueueProjectDeletionSweep(ctx, 0)
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.enqueueProjectQuotaSweep(context.Background(), 0)
				s.enqueueProjectDeletionSweep(context.Background(), 0)
			}
		}
	}()
}

func decodeTaskPayload[T any](task *asynq.Task) (T, error) {
	var payload T
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func (s *Server) handleProvisionProjectTask(ctx context.Context, task *asynq.Task) error {
	payload, err := decodeTaskPayload[projectJobPayload](task)
	if err != nil {
		return err
	}
	project, err := s.store.ProjectForUser(ctx, payload.UserID, payload.ProjectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if err := s.provisionProject(ctx, payload.UserID, payload.UserEmail, project, payload.Prompt); err != nil {
		s.recordProjectProvisionFailure(ctx, payload.UserID, project, err)
		return err
	}
	return nil
}

func (s *Server) handleRecoverProjectTask(ctx context.Context, task *asynq.Task) error {
	payload, err := decodeTaskPayload[projectJobPayload](task)
	if err != nil {
		return err
	}
	project, err := s.store.ProjectForUser(ctx, payload.UserID, payload.ProjectID)
	if err != nil || !projectNeedsReadinessRecovery(project) {
		return nil
	}
	fibe, err := s.fibeClientForProject(ctx, project, payload.UserEmail)
	if err != nil {
		return err
	}
	return s.recoverProjectReadiness(ctx, payload.UserID, project, fibe)
}

func (s *Server) handleDeleteProjectResourcesTask(ctx context.Context, task *asynq.Task) error {
	payload, err := decodeTaskPayload[projectJobPayload](task)
	if err != nil {
		return err
	}
	project, err := s.store.ProjectForUser(ctx, payload.UserID, payload.ProjectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if projectHasFibeResources(project) {
		fibe, err := s.fibeClientForProject(ctx, project, payload.UserEmail)
		if err != nil {
			return err
		}
		if err := fibe.DeleteProjectResources(ctx, project); err != nil {
			return err
		}
	}
	if err := s.deleteLocalProjectAttachmentDirs([]Project{*project}); err != nil {
		return err
	}
	if err := s.store.DeleteProject(ctx, project.ID, payload.UserID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

func (s *Server) handleProjectDeletionSweepTask(ctx context.Context, _ *asynq.Task) error {
	projects, err := s.store.DeletingProjects(ctx, 100)
	if err != nil {
		return err
	}
	for i := range projects {
		project := &projects[i]
		user, err := s.store.UserByID(ctx, project.UserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return err
		}
		if err := s.enqueueProjectJob(ctx, taskDeleteProjectResources, projectJobPayload{UserID: user.ID, UserEmail: user.Email, ProjectID: project.ID}, asynq.Queue("critical"), asynq.MaxRetry(10), asynq.Timeout(20*time.Minute), asynq.Unique(time.Minute)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleArchiveDeleteProjectTask(ctx context.Context, task *asynq.Task) error {
	payload, err := decodeTaskPayload[projectJobPayload](task)
	if err != nil {
		return err
	}
	user, err := s.store.UserByID(ctx, payload.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	project, err := s.store.ProjectForUser(ctx, payload.UserID, payload.ProjectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	archive, err := s.archiveProjectSource(ctx, user, project)
	if err != nil {
		return err
	}
	if conn, err := s.store.SocialConnection(ctx, user.ID, "github"); err == nil {
		repoName := projecttext.SourceName(project.Title) + "-archive-" + strings.ReplaceAll(project.ID[:min(len(project.ID), 8)], "-", "")
		if repoURL, err := s.exportProjectToGithub(ctx, user, project, conn, repoName, true); err == nil {
			archive.GithubRepoURL = repoURL
			_ = s.store.UpsertProjectArchive(ctx, archive)
			s.notifyProjectExportReady(ctx, user, project, repoURL)
		} else {
			log.Printf("archive project %s github export failed: %v", project.ID, err)
		}
	}
	s.notifyProjectArchiveReady(ctx, user, project.Title, archive.DownloadURL, parseTimeOrZero(archive.ExpiresAt))
	if err := s.store.UpdateProjectStatus(ctx, project.ID, user.ID, "deleting"); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return s.enqueueProjectJob(ctx, taskDeleteProjectResources, projectJobPayload{UserID: user.ID, UserEmail: user.Email, ProjectID: project.ID}, asynq.Queue("critical"), asynq.MaxRetry(10), asynq.Timeout(20*time.Minute))
}

func (s *Server) handleSendEmailTask(ctx context.Context, task *asynq.Task) error {
	payload, err := decodeTaskPayload[emailJobPayload](task)
	if err != nil {
		return err
	}
	return s.sendEmail(ctx, emailMessage{To: payload.To, Subject: payload.Subject, Body: payload.Body})
}

func (s *Server) handleProjectQuotaSweepTask(ctx context.Context, _ *asynq.Task) error {
	users, err := s.store.UsersWithProjects(ctx)
	if err != nil {
		return err
	}
	for i := range users {
		user := &users[i]
		limit := s.projectCapForUser(ctx, user)
		excess, err := s.store.ProjectsExceedingQuota(ctx, user.ID, limit)
		if err != nil {
			return err
		}
		for j := range excess {
			project := &excess[j]
			if err := s.enqueueProjectJob(ctx, taskArchiveDeleteProject, projectJobPayload{UserID: user.ID, UserEmail: user.Email, ProjectID: project.ID, Reason: "project quota exceeded"}, asynq.Queue("critical"), asynq.MaxRetry(10), asynq.Timeout(20*time.Minute), asynq.Unique(24*time.Hour)); err != nil {
				return err
			}
		}
	}
	return s.cleanupExpiredArchives(ctx)
}

func (s *Server) cleanupExpiredArchives(ctx context.Context) error {
	archives, err := s.store.ExpiredArchives(ctx, 100)
	if err != nil {
		return err
	}
	for _, archive := range archives {
		if archive.StoragePath != "" {
			target := filepath.Clean(filepath.Join(s.store.DataDir(), archive.StoragePath))
			if strings.HasPrefix(target, filepath.Clean(filepath.Join(s.store.DataDir(), "archives"))+string(os.PathSeparator)) {
				_ = os.Remove(target)
			}
		}
		if err := s.store.DeleteProjectArchive(ctx, archive.ID); err != nil {
			return err
		}
	}
	return nil
}

func parseTimeOrZero(raw string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	return parsed
}
