package store

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"strings"
)

func (s *Store) CreateExportJob(ctx context.Context, projectID string) (string, error) {
	id := uuid.NewString()
	now := nowString()
	_, err := s.db.ExecContext(ctx, `INSERT INTO export_jobs(id, project_id, status, created_at, updated_at) VALUES(?, ?, 'running', ?, ?)`, id, projectID, now, now)
	return id, err
}

func (s *Store) FinishExportJob(ctx context.Context, id, status, repoURL, errText string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE export_jobs SET status = ?, target_repo_url = ?, error = ?, updated_at = ? WHERE id = ?`, status, repoURL, errText, nowString(), id)
	return err
}

func (s *Store) MustConfig(ctx context.Context, key string) (string, error) {
	cfg, err := s.ConfigMap(ctx)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(cfg[key])
	if value == "" {
		return "", fmt.Errorf("%s is not configured", key)
	}
	return value, nil
}

func (s *Store) UpsertProjectArchive(ctx context.Context, archive *ProjectArchive) error {
	if archive.ID == "" {
		archive.ID = uuid.NewString()
	}
	now := nowString()
	if archive.CreatedAt == "" {
		archive.CreatedAt = now
	}
	archive.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO project_archives(id, user_id, project_id, project_title, storage_path, status, github_repo_url, error, expires_at, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_title=excluded.project_title,
			storage_path=excluded.storage_path,
			status=excluded.status,
			github_repo_url=excluded.github_repo_url,
			error=excluded.error,
			expires_at=excluded.expires_at,
			updated_at=excluded.updated_at
	`, archive.ID, archive.UserID, archive.ProjectID, archive.ProjectTitle, archive.StoragePath, archive.Status, archive.GithubRepoURL, archive.Error, archive.ExpiresAt, archive.CreatedAt, archive.UpdatedAt)
	return err
}

func (s *Store) ArchivesForUser(ctx context.Context, userID string) ([]ProjectArchive, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, project_id, project_title, storage_path, status, github_repo_url, error, expires_at, created_at, updated_at
		FROM project_archives
		WHERE user_id = ? AND expires_at > ?
		ORDER BY created_at DESC
	`, userID, nowString())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjectArchives(rows)
}

func (s *Store) ArchiveForUser(ctx context.Context, userID, archiveID string) (*ProjectArchive, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, project_id, project_title, storage_path, status, github_repo_url, error, expires_at, created_at, updated_at
		FROM project_archives
		WHERE user_id = ? AND id = ? AND expires_at > ?
	`, userID, archiveID, nowString())
	var archive ProjectArchive
	if err := row.Scan(&archive.ID, &archive.UserID, &archive.ProjectID, &archive.ProjectTitle, &archive.StoragePath, &archive.Status, &archive.GithubRepoURL, &archive.Error, &archive.ExpiresAt, &archive.CreatedAt, &archive.UpdatedAt); err != nil {
		return nil, err
	}
	return &archive, nil
}

func (s *Store) LatestProjectArchive(ctx context.Context, userID, projectID string) (*ProjectArchive, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, project_id, project_title, storage_path, status, github_repo_url, error, expires_at, created_at, updated_at
		FROM project_archives
		WHERE user_id = ? AND project_id = ? AND status = 'ready' AND expires_at > ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, projectID, nowString())
	var archive ProjectArchive
	if err := row.Scan(&archive.ID, &archive.UserID, &archive.ProjectID, &archive.ProjectTitle, &archive.StoragePath, &archive.Status, &archive.GithubRepoURL, &archive.Error, &archive.ExpiresAt, &archive.CreatedAt, &archive.UpdatedAt); err != nil {
		return nil, err
	}
	return &archive, nil
}

func (s *Store) ExpiredArchives(ctx context.Context, limit int) ([]ProjectArchive, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, project_id, project_title, storage_path, status, github_repo_url, error, expires_at, created_at, updated_at
		FROM project_archives
		WHERE expires_at <= ?
		ORDER BY expires_at ASC
		LIMIT ?
	`, nowString(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProjectArchives(rows)
}

func scanProjectArchives(rows *sql.Rows) ([]ProjectArchive, error) {
	var out []ProjectArchive
	for rows.Next() {
		var archive ProjectArchive
		if err := rows.Scan(&archive.ID, &archive.UserID, &archive.ProjectID, &archive.ProjectTitle, &archive.StoragePath, &archive.Status, &archive.GithubRepoURL, &archive.Error, &archive.ExpiresAt, &archive.CreatedAt, &archive.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, archive)
	}
	if out == nil {
		out = []ProjectArchive{}
	}
	return out, rows.Err()
}

func (s *Store) DeleteProjectArchive(ctx context.Context, archiveID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM project_archives WHERE id = ?`, archiveID)
	return err
}
