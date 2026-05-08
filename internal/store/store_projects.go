package store

import (
	"context"
	"database/sql"
	"strings"
)

func (s *Store) CreateProject(ctx context.Context, project *Project) error {
	now := nowString()
	project.CreatedAt = now
	project.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects(id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playspec_id, prop_id, repo_url, preview_url, status, error_message, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, project.ID, project.UserID, project.Title, project.ConversationID, project.AgentID, project.MarqueeID, project.PlaygroundID, project.PlayspecID, project.PropID, project.RepoURL, project.PreviewURL, project.Status, project.ErrorMessage, project.CreatedAt, project.UpdatedAt)
	return err
}

func (s *Store) ProjectCountForUser(ctx context.Context, userID string) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE user_id = ? AND status != 'deleting'`, userID)
	var count int
	return count, row.Scan(&count)
}

func (s *Store) UpdateProjectProvisioning(ctx context.Context, projectID, userID, playgroundID, playspecID, propID, repoURL, previewURL, status string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET playground_id = ?, playspec_id = ?, prop_id = ?, repo_url = ?, preview_url = ?, status = ?, error_message = '', updated_at = ?
		WHERE id = ? AND user_id = ?
	`, playgroundID, playspecID, propID, repoURL, previewURL, status, nowString(), projectID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateProjectError(ctx context.Context, projectID, userID, message string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET status = 'error', error_message = ?, updated_at = ?
		WHERE id = ? AND user_id = ?
	`, publicProjectErrorMessage(message), nowString(), projectID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func publicProjectErrorMessage(message string) string {
	raw := strings.ToLower(strings.TrimSpace(message))
	switch {
	case raw == "":
		return "The canvas could not start. Ask an admin to check workspace settings, then create a new project."
	case strings.Contains(raw, "not configured"):
		return "Workspace settings are incomplete. Ask an admin to review the configuration, then create a new project."
	case strings.Contains(raw, "timed out") || strings.Contains(raw, "timeout") || strings.Contains(raw, "did not become ready"):
		return "The canvas took too long to start. Try creating a new project."
	default:
		return "The canvas could not start. Ask an admin to check workspace settings, then create a new project."
	}
}

func (s *Store) UpdateProjectStatus(ctx context.Context, projectID, userID, status string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET status = ?, updated_at = ?
		WHERE id = ? AND user_id = ?
	`, status, nowString(), projectID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateProjectTitle(ctx context.Context, projectID, userID, title string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET title = ?, updated_at = ?
		WHERE id = ? AND user_id = ?
	`, title, nowString(), projectID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteProject(ctx context.Context, projectID, userID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ? AND user_id = ?`, projectID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) ProjectForUser(ctx context.Context, userID, projectID string) (*Project, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playspec_id, prop_id, repo_url, preview_url, status, error_message, created_at, updated_at
		FROM projects
		WHERE id = ? AND user_id = ?
	`, projectID, userID)
	return scanProject(row)
}

func (s *Store) AllProjectsForUser(ctx context.Context, userID string) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playspec_id, prop_id, repo_url, preview_url, status, error_message, created_at, updated_at
		FROM projects
		WHERE user_id = ?
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *project)
	}
	if out == nil {
		out = []Project{}
	}
	return out, rows.Err()
}

func (s *Store) ProjectsForUser(ctx context.Context, userID string) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playspec_id, prop_id, repo_url, preview_url, status, error_message, created_at, updated_at
		FROM projects
		WHERE user_id = ? AND status != 'deleting'
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *project)
	}
	if out == nil {
		out = []Project{}
	}
	return out, rows.Err()
}

func scanProject(scanner interface{ Scan(...any) error }) (*Project, error) {
	var project Project
	if err := scanner.Scan(&project.ID, &project.UserID, &project.Title, &project.ConversationID, &project.AgentID, &project.MarqueeID, &project.PlaygroundID, &project.PlayspecID, &project.PropID, &project.RepoURL, &project.PreviewURL, &project.Status, &project.ErrorMessage, &project.CreatedAt, &project.UpdatedAt); err != nil {
		return nil, err
	}
	return &project, nil
}

func (s *Store) UsersWithProjects(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT users.id, users.email, users.name, users.avatar_url, users.access_status, users.access_note, users.created_at
		FROM users
		JOIN projects ON projects.user_id = users.id AND projects.status != 'deleting'
		ORDER BY users.created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *user)
	}
	if out == nil {
		out = []User{}
	}
	return out, rows.Err()
}

func (s *Store) ProjectsExceedingQuota(ctx context.Context, userID string, limit int) ([]Project, error) {
	if limit < 0 {
		limit = 0
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT projects.id, projects.user_id, projects.title, projects.conversation_id, projects.agent_id, projects.marquee_id,
			projects.playground_id, projects.playspec_id, projects.prop_id, projects.repo_url, projects.preview_url,
			projects.status, projects.error_message, projects.created_at, projects.updated_at,
			COALESCE(MAX(messages.created_at), projects.updated_at) AS last_activity_at
		FROM projects
		LEFT JOIN messages ON messages.project_id = projects.id AND messages.role = 'user'
		WHERE projects.user_id = ? AND projects.status != 'deleting'
		GROUP BY projects.id
		ORDER BY last_activity_at DESC, projects.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []Project
	for rows.Next() {
		var project Project
		var lastActivity string
		if err := rows.Scan(&project.ID, &project.UserID, &project.Title, &project.ConversationID, &project.AgentID, &project.MarqueeID, &project.PlaygroundID, &project.PlayspecID, &project.PropID, &project.RepoURL, &project.PreviewURL, &project.Status, &project.ErrorMessage, &project.CreatedAt, &project.UpdatedAt, &lastActivity); err != nil {
			return nil, err
		}
		all = append(all, project)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(all) <= limit {
		return []Project{}, nil
	}
	return all[limit:], nil
}
