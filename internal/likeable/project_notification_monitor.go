package likeable

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/hibiken/asynq"
)

const projectNotificationMonitorInterval = 3 * time.Second

func (s *Server) enqueueProjectNotificationMonitor(ctx context.Context, userID, userEmail, projectID string, delay time.Duration) {
	if userID == "" || projectID == "" {
		return
	}
	if s.jobs == nil {
		return
	}
	payload := projectJobPayload{UserID: userID, UserEmail: userEmail, ProjectID: projectID}
	opts := []asynq.Option{asynq.Queue("low"), asynq.MaxRetry(2), asynq.Timeout(45 * time.Second), asynq.Unique(2 * time.Second)}
	if delay > 0 {
		opts = append(opts, asynq.ProcessIn(delay))
	}
	if err := s.enqueueProjectJob(ctx, taskMonitorProjectNotifications, payload, opts...); err != nil {
		log.Printf("enqueue project notification monitor %s: %v", projectID, err)
	}
}

func (s *Server) handleMonitorProjectNotificationsTask(ctx context.Context, task *asynq.Task) error {
	payload, err := decodeTaskPayload[projectJobPayload](task)
	if err != nil {
		return err
	}
	return s.runProjectNotificationMonitor(ctx, payload)
}

func (s *Server) runProjectNotificationMonitor(ctx context.Context, payload projectJobPayload) error {
	project, err := s.store.ProjectForUser(ctx, payload.UserID, payload.ProjectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if project.Status == "deleting" {
		return nil
	}
	if s.projectFeedForegroundRecent(project.ID, projectFeedForegroundTTL) {
		s.enqueueProjectNotificationMonitor(context.Background(), payload.UserID, payload.UserEmail, payload.ProjectID, projectFeedForegroundDelay)
		return nil
	}
	shouldContinue, err := s.refreshProjectNotificationTimings(ctx, project, payload.UserEmail)
	if err != nil {
		return err
	}
	if shouldContinue {
		s.enqueueProjectNotificationMonitor(context.Background(), payload.UserID, payload.UserEmail, payload.ProjectID, projectNotificationMonitorInterval)
	}
	return nil
}

func (s *Server) refreshProjectNotificationTimings(ctx context.Context, project *Project, userEmail string) (bool, error) {
	snapshot, err := s.loadProjectFeedSnapshot(ctx, &User{ID: project.UserID, Email: userEmail}, project, false)
	if err != nil {
		return false, err
	}
	if snapshot == nil {
		return false, nil
	}
	return snapshot.shouldMonitor, nil
}
