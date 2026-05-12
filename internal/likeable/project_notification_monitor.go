package likeable

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	fibegateway "github.com/fibegg/likeable/internal/fibe"
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
	local, err := s.store.MessagesForProject(ctx, project.ID)
	if err != nil {
		return false, err
	}
	fibeClient, err := s.fibeClientForProject(ctx, project, userEmail)
	if err != nil {
		return false, err
	}
	messages, messagesErr := fibeClient.Messages(ctx, project.ConversationID)
	activity, activityErr := fibeClient.Activity(ctx, project.ConversationID)
	live, liveErr := fibeClient.ConversationLiveState(ctx, project.ConversationID)
	if messagesErr != nil {
		if fibegateway.IsConversationMissingError(messagesErr) {
			messages = []any{}
		} else {
			log.Printf("monitor project feed messages for project %s: %v", project.ID, messagesErr)
			messages = []any{}
		}
	}
	if activityErr != nil {
		if fibegateway.IsConversationMissingError(activityErr) {
			activity = []any{}
		} else {
			log.Printf("monitor project feed activity for project %s: %v", project.ID, activityErr)
			activity = []any{}
		}
	}
	if liveErr != nil {
		if fibegateway.IsConversationMissingError(liveErr) {
			live = &fibegateway.ConversationLiveState{ConversationID: project.ConversationID}
		} else {
			return false, liveErr
		}
	}
	if messages == nil {
		messages = []any{}
	}
	if activity == nil {
		activity = []any{}
	}
	messages = sanitizeAgentProtocolMessages(messages)
	sanitizeAgentProtocolLiveState(live)
	_, shouldContinue, err := s.syncProjectNotificationTimings(ctx, project, local, messages, activity, live)
	return shouldContinue, err
}
