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
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	startedAt = startedAt.UTC()
	now := nowString()
	startedRaw := startedAt.Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO project_notification_timings(project_id, notification_id, body, started_at, updated_at)
		VALUES(?, ?, ?, ?, ?)
	`, projectID, notificationID, body, startedRaw, now)
	if err != nil {
		return err
	}
	if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
		return nil
	}

	var existingStartedRaw, completedRaw string
	if err := s.db.QueryRowContext(ctx, `
		SELECT started_at, completed_at
		FROM project_notification_timings
		WHERE project_id = ? AND notification_id = ?
	`, projectID, notificationID).Scan(&existingStartedRaw, &completedRaw); err != nil {
		return err
	}
	nextStartedRaw := existingStartedRaw
	if completedRaw == "" {
		existingStartedAt, ok := parseNotificationTimingTime(existingStartedRaw)
		if !ok || startedAt.Before(existingStartedAt) {
			nextStartedRaw = startedRaw
		}
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE project_notification_timings
		SET body = ?, started_at = ?, updated_at = ?
		WHERE project_id = ? AND notification_id = ?
	`, body, nextStartedRaw, now, projectID, notificationID)
	return err
}

func (s *Store) CompleteProjectNotificationTiming(ctx context.Context, projectID, notificationID string, completedAt time.Time) error {
	return s.completeProjectNotificationTiming(ctx, projectID, notificationID, completedAt, nil)
}

func (s *Store) CompleteProjectNotificationTimingWithElapsed(ctx context.Context, projectID, notificationID string, completedAt time.Time, elapsedMs int64) error {
	if elapsedMs < 0 {
		elapsedMs = 0
	}
	return s.completeProjectNotificationTiming(ctx, projectID, notificationID, completedAt, &elapsedMs)
}

func (s *Store) completeProjectNotificationTiming(ctx context.Context, projectID, notificationID string, completedAt time.Time, elapsedOverrideMs *int64) error {
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
	startedAt, ok := parseNotificationTimingTime(startedRaw)
	if !ok {
		startedAt = completedAt
	}
	if completedAt.Before(startedAt) {
		completedAt = startedAt
	}
	elapsedMs := completedAt.Sub(startedAt).Milliseconds()
	if elapsedOverrideMs != nil {
		elapsedMs = *elapsedOverrideMs
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE project_notification_timings
		SET completed_at = ?, elapsed_ms = ?, updated_at = ?
		WHERE project_id = ? AND notification_id = ? AND completed_at = ''
	`, completedAt.UTC().Format(time.RFC3339Nano), elapsedMs, nowString(), projectID, notificationID)
	return err
}

func parseNotificationTimingTime(raw string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, raw)
	}
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}
