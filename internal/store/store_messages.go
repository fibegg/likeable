package store

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"time"
)

func (s *Store) AddMessage(ctx context.Context, projectID, role, body string) (*Message, error) {
	return s.AddMessageAt(ctx, projectID, role, body, nowString())
}

func (s *Store) AddMessageAt(ctx context.Context, projectID, role, body, createdAt string) (*Message, error) {
	return s.AddMessageWithAttachments(ctx, &Message{
		ID:        uuid.NewString(),
		ProjectID: projectID,
		Role:      role,
		Body:      body,
		CreatedAt: createdAt,
	}, nil)
}

func (s *Store) AddMessageWithAttachments(ctx context.Context, msg *Message, attachments []MessageAttachment) (*Message, error) {
	if msg.ID == "" {
		msg.ID = uuid.NewString()
	}
	if msg.CreatedAt == "" {
		msg.CreatedAt = nowString()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO messages(id, project_id, role, body, created_at)
		VALUES(?, ?, ?, ?, ?)
	`, msg.ID, msg.ProjectID, msg.Role, msg.Body, msg.CreatedAt); err != nil {
		return nil, err
	}
	for i := range attachments {
		attachment := &attachments[i]
		if attachment.ID == "" {
			attachment.ID = uuid.NewString()
		}
		attachment.MessageID = msg.ID
		attachment.ProjectID = msg.ProjectID
		if attachment.CreatedAt == "" {
			attachment.CreatedAt = msg.CreatedAt
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO message_attachments(id, message_id, project_id, filename, content_type, size, storage_path, created_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		`, attachment.ID, attachment.MessageID, attachment.ProjectID, attachment.Filename, attachment.ContentType, attachment.Size, attachment.StoragePath, attachment.CreatedAt); err != nil {
			return nil, err
		}
		attachment.URL = messageAttachmentURL(attachment.ProjectID, attachment.ID)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	msg.Attachments = attachments
	return msg, nil
}

func (s *Store) MessagesForProject(ctx context.Context, projectID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, role, body, created_at FROM messages WHERE project_id = ? ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.ProjectID, &msg.Role, &msg.Body, &msg.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	attachments, err := s.attachmentsForProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	byMessage := map[string][]MessageAttachment{}
	for _, attachment := range attachments {
		byMessage[attachment.MessageID] = append(byMessage[attachment.MessageID], attachment)
	}
	for i := range out {
		out[i].Attachments = byMessage[out[i].ID]
	}
	return out, nil
}

func (s *Store) attachmentsForProject(ctx context.Context, projectID string) ([]MessageAttachment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, message_id, project_id, filename, content_type, size, storage_path, created_at
		FROM message_attachments
		WHERE project_id = ?
		ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MessageAttachment
	for rows.Next() {
		var attachment MessageAttachment
		if err := rows.Scan(&attachment.ID, &attachment.MessageID, &attachment.ProjectID, &attachment.Filename, &attachment.ContentType, &attachment.Size, &attachment.StoragePath, &attachment.CreatedAt); err != nil {
			return nil, err
		}
		attachment.URL = messageAttachmentURL(attachment.ProjectID, attachment.ID)
		out = append(out, attachment)
	}
	return out, rows.Err()
}

func (s *Store) AttachmentForProject(ctx context.Context, projectID, attachmentID string) (*MessageAttachment, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, message_id, project_id, filename, content_type, size, storage_path, created_at
		FROM message_attachments
		WHERE project_id = ? AND id = ?
	`, projectID, attachmentID)
	var attachment MessageAttachment
	if err := row.Scan(&attachment.ID, &attachment.MessageID, &attachment.ProjectID, &attachment.Filename, &attachment.ContentType, &attachment.Size, &attachment.StoragePath, &attachment.CreatedAt); err != nil {
		return nil, err
	}
	attachment.URL = messageAttachmentURL(attachment.ProjectID, attachment.ID)
	return &attachment, nil
}

func messageAttachmentURL(projectID, attachmentID string) string {
	return fmt.Sprintf("/api/projects/%s/attachments/%s", projectID, attachmentID)
}

func (s *Store) UserMessageCount(ctx context.Context, userID string) (int, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM messages
		JOIN projects ON projects.id = messages.project_id
		WHERE projects.user_id = ? AND messages.role = 'user'
	`, userID)
	var count int
	return count, row.Scan(&count)
}

func (s *Store) UserMessageCountSince(ctx context.Context, userID string, since time.Time) (int, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM messages
		JOIN projects ON projects.id = messages.project_id
		WHERE projects.user_id = ? AND messages.role = 'user' AND messages.created_at >= ?
	`, userID, since.UTC().Format(time.RFC3339Nano))
	var count int
	return count, row.Scan(&count)
}
