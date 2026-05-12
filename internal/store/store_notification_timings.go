package store

import (
	"context"
	"time"
)

func (s *Store) ProjectNotificationTimingMap(ctx context.Context, projectID string) (map[string]ProjectNotificationTiming, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT project_id, notification_id, body, started_at, completed_at, elapsed_ms, updated_at
		FROM project_notification_timings
		WHERE project_id = ?
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ProjectNotificationTiming{}
	for rows.Next() {
		var timing ProjectNotificationTiming
		if err := rows.Scan(&timing.ProjectID, &timing.NotificationID, &timing.Body, &timing.StartedAt, &timing.CompletedAt, &timing.ElapsedMs, &timing.UpdatedAt); err != nil {
			return nil, err
		}
		out[timing.NotificationID] = timing
	}
	return out, rows.Err()
}

func (s *Store) UpsertProjectNotificationStarted(ctx context.Context, projectID, notificationID, body string, startedAt time.Time) error {
	now := nowString()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO project_notification_timings(project_id, notification_id, body, started_at, updated_at)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(project_id, notification_id) DO UPDATE SET
			body = excluded.body,
			updated_at = excluded.updated_at
	`, projectID, notificationID, body, startedAt.UTC().Format(time.RFC3339Nano), now)
	return err
}

func (s *Store) CompleteProjectNotificationTiming(ctx context.Context, projectID, notificationID string, completedAt time.Time) error {
	row := s.db.QueryRowContext(ctx, `
		SELECT started_at, completed_at
		FROM project_notification_timings
		WHERE project_id = ? AND notification_id = ?
	`, projectID, notificationID)
	var startedRaw, completedRaw string
	if err := row.Scan(&startedRaw, &completedRaw); err != nil {
		return err
	}
	if completedRaw != "" {
		return nil
	}
	startedAt, err := time.Parse(time.RFC3339Nano, startedRaw)
	if err != nil {
		startedAt = completedAt
	}
	if completedAt.Before(startedAt) {
		completedAt = startedAt
	}
	elapsedMs := completedAt.Sub(startedAt).Milliseconds()
	_, err = s.db.ExecContext(ctx, `
		UPDATE project_notification_timings
		SET completed_at = ?, elapsed_ms = ?, updated_at = ?
		WHERE project_id = ? AND notification_id = ? AND completed_at = ''
	`, completedAt.UTC().Format(time.RFC3339Nano), elapsedMs, nowString(), projectID, notificationID)
	return err
}
