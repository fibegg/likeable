package likeable

import (
	"context"
	"github.com/hibiken/asynq"
	"log"
	"time"
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
		if snapshot.PlaygroundID != "" || snapshot.PlayspecID != "" || snapshot.PropID != "" || snapshot.RepoURL != "" || snapshot.ConversationID != "" {
			fibe, err := s.fibeClientForProject(ctx, &snapshot, userEmail)
			if err != nil {
				log.Printf("delete project %s resources: %v", snapshot.ID, err)
				return
			}
			if err := fibe.DeleteProjectResources(ctx, &snapshot); err != nil {
				log.Printf("delete project %s resources: %v", snapshot.ID, err)
				return
			}
		}
		if err := s.store.DeleteProject(ctx, snapshot.ID, userID); err != nil {
			log.Printf("delete local project %s: %v", snapshot.ID, err)
		}
	}()
}
