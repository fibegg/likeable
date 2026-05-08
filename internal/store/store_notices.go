package store

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"strings"
	"time"
)

func (s *Store) AddUserNotice(ctx context.Context, notice UserNotice) (*UserNotice, error) {
	if notice.ID == "" {
		notice.ID = uuid.NewString()
	}
	notice.UserID = strings.TrimSpace(notice.UserID)
	notice.Sender = normalizeNoticeSender(notice.Sender)
	notice.Severity = normalizeNoticeSeverity(notice.Severity)
	notice.Body = strings.TrimSpace(notice.Body)
	if notice.Body == "" {
		return nil, errors.New("notice body is required")
	}
	if notice.CreatedAt == "" {
		notice.CreatedAt = nowString()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_notices(id, user_id, sender, severity, body, read_at, dismissed_at, unsent_at, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, notice.ID, notice.UserID, notice.Sender, notice.Severity, notice.Body, notice.ReadAt, notice.DismissedAt, notice.UnsentAt, notice.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &notice, nil
}

func normalizeNoticeSender(sender string) string {
	switch strings.ToLower(strings.TrimSpace(sender)) {
	case "system", "user", "admin":
		return strings.ToLower(strings.TrimSpace(sender))
	default:
		return "admin"
	}
}

func normalizeNoticeSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "warning", "danger", "info":
		return strings.ToLower(strings.TrimSpace(severity))
	default:
		return "info"
	}
}

func (s *Store) NoticesForUser(ctx context.Context, userID string, limit int) ([]UserNotice, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, sender, severity, body, read_at, dismissed_at, unsent_at, created_at
		FROM user_notices
		WHERE user_id = ? AND unsent_at = ''
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotices(rows)
}

func (s *Store) ActiveNoticesForUser(ctx context.Context, userID string, limit int) ([]UserNotice, error) {
	if limit <= 0 || limit > 10 {
		limit = 3
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, sender, severity, body, read_at, dismissed_at, unsent_at, created_at
		FROM user_notices
		WHERE user_id = ? AND unsent_at = '' AND dismissed_at = '' AND sender IN ('admin', 'system')
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotices(rows)
}

func (s *Store) NoticeExistsSince(ctx context.Context, userID, sender, bodyPrefix string, since time.Time) (bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM user_notices
		WHERE user_id = ? AND sender = ? AND body LIKE ? AND created_at >= ? AND unsent_at = ''
	`, userID, normalizeNoticeSender(sender), strings.TrimSpace(bodyPrefix)+"%", since.UTC().Format(time.RFC3339Nano))
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func scanNotices(rows *sql.Rows) ([]UserNotice, error) {
	var out []UserNotice
	for rows.Next() {
		var notice UserNotice
		if err := rows.Scan(&notice.ID, &notice.UserID, &notice.Sender, &notice.Severity, &notice.Body, &notice.ReadAt, &notice.DismissedAt, &notice.UnsentAt, &notice.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, notice)
	}
	if out == nil {
		out = []UserNotice{}
	}
	return out, rows.Err()
}

func (s *Store) DismissUserNotice(ctx context.Context, userID, noticeID string) (*UserNotice, error) {
	now := nowString()
	result, err := s.db.ExecContext(ctx, `
		UPDATE user_notices
		SET dismissed_at = CASE WHEN dismissed_at = '' THEN ? ELSE dismissed_at END,
			read_at = CASE WHEN read_at = '' THEN ? ELSE read_at END
		WHERE user_id = ? AND id = ? AND unsent_at = ''
	`, now, now, userID, noticeID)
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, sql.ErrNoRows
	}
	return s.UserNotice(ctx, userID, noticeID)
}

func (s *Store) MarkUserNoticeRead(ctx context.Context, userID, noticeID string) (*UserNotice, error) {
	now := nowString()
	result, err := s.db.ExecContext(ctx, `
		UPDATE user_notices
		SET read_at = CASE WHEN read_at = '' THEN ? ELSE read_at END
		WHERE user_id = ? AND id = ? AND unsent_at = ''
	`, now, userID, noticeID)
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, sql.ErrNoRows
	}
	return s.UserNotice(ctx, userID, noticeID)
}

func (s *Store) UnsendUserNotice(ctx context.Context, userID, noticeID string) (*UserNotice, error) {
	now := nowString()
	result, err := s.db.ExecContext(ctx, `
		UPDATE user_notices
		SET unsent_at = CASE WHEN unsent_at = '' THEN ? ELSE unsent_at END
		WHERE user_id = ? AND id = ? AND sender IN ('admin', 'system')
	`, now, userID, noticeID)
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, sql.ErrNoRows
	}
	return s.UserNoticeIncludingUnsent(ctx, userID, noticeID)
}

func (s *Store) UserNotice(ctx context.Context, userID, noticeID string) (*UserNotice, error) {
	return s.userNotice(ctx, userID, noticeID, false)
}

func (s *Store) UserNoticeIncludingUnsent(ctx context.Context, userID, noticeID string) (*UserNotice, error) {
	return s.userNotice(ctx, userID, noticeID, true)
}

func (s *Store) userNotice(ctx context.Context, userID, noticeID string, includeUnsent bool) (*UserNotice, error) {
	query := `
		SELECT id, user_id, sender, severity, body, read_at, dismissed_at, unsent_at, created_at
		FROM user_notices
		WHERE user_id = ? AND id = ?
	`
	if !includeUnsent {
		query += ` AND unsent_at = ''`
	}
	var notice UserNotice
	if err := s.db.QueryRowContext(ctx, query, userID, noticeID).Scan(&notice.ID, &notice.UserID, &notice.Sender, &notice.Severity, &notice.Body, &notice.ReadAt, &notice.DismissedAt, &notice.UnsentAt, &notice.CreatedAt); err != nil {
		return nil, err
	}
	return &notice, nil
}
