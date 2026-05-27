package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Store) CreateProject(ctx context.Context, project *Project) error {
	now := nowString()
	project.CreatedAt = now
	project.UpdatedAt = now
	if strings.TrimSpace(project.PlaygroundLastUsedAt) == "" {
		project.PlaygroundLastUsedAt = now
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO projects(id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playground_name, playspec_id, prop_id, repo_url, preview_url, selected_service_name, status, error_message, provisioning_lock_until, cleanup_last_error, playground_last_used_at, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, project.ID, project.UserID, project.Title, project.ConversationID, project.AgentID, project.MarqueeID, project.PlaygroundID, project.PlaygroundName, project.PlayspecID, project.PropID, project.RepoURL, project.PreviewURL, project.SelectedService, project.Status, project.ErrorMessage, project.ProvisioningLockUntil, project.CleanupLastError, project.PlaygroundLastUsedAt, project.CreatedAt, project.UpdatedAt)
	return err
}

func (s *Store) ProjectCountForUser(ctx context.Context, userID string) (int, error) {
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE user_id = ? AND status NOT IN ('deleting', 'archived')`, userID)
	var count int
	return count, row.Scan(&count)
}

func (s *Store) UpdateProjectProvisioning(ctx context.Context, projectID, userID, playgroundID, playspecID, propID, repoURL, previewURL, selectedServiceName, status string) error {
	now := nowString()
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET playground_id = ?, playspec_id = ?, prop_id = ?, repo_url = ?, preview_url = ?, selected_service_name = ?, status = ?, error_message = '', internal_error_message = '',
			playground_last_used_at = CASE WHEN ? = 'ready' AND playground_last_used_at = '' THEN ? ELSE playground_last_used_at END,
			updated_at = ?
		WHERE id = ? AND user_id = ? AND status != 'deleting'
	`, playgroundID, playspecID, propID, repoURL, previewURL, selectedServiceName, status, status, now, now, projectID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) TryAcquireProjectProvisioning(ctx context.Context, projectID, userID string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	now := nowString()
	lockUntil := time.Now().UTC().Add(ttl).Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET provisioning_lock_until = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND status != 'deleting' AND (provisioning_lock_until = '' OR provisioning_lock_until <= ?)
	`, lockUntil, now, projectID, userID, now)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (s *Store) ClearProjectProvisioningLease(ctx context.Context, projectID, userID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET provisioning_lock_until = '', updated_at = ?
		WHERE id = ? AND user_id = ?
	`, nowString(), projectID, userID)
	return err
}

func (s *Store) UpdateProjectCleanupError(ctx context.Context, projectID, userID, message string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET cleanup_last_error = ?, updated_at = ?
		WHERE id = ? AND user_id = ?
	`, message, nowString(), projectID, userID)
	return err
}

func (s *Store) TryAcquireProjectCleanup(ctx context.Context, projectID, userID string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	now := nowString()
	lockUntil := time.Now().UTC().Add(ttl).Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET cleanup_lock_until = ?, cleanup_last_error = '', updated_at = ?
		WHERE id = ? AND user_id = ? AND status IN ('deleting', 'archived') AND (cleanup_lock_until = '' OR cleanup_lock_until <= ?)
	`, lockUntil, now, projectID, userID, now)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (s *Store) ClearProjectCleanupLease(ctx context.Context, projectID, userID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET cleanup_lock_until = '', updated_at = ?
		WHERE id = ? AND user_id = ?
	`, nowString(), projectID, userID)
	return err
}

func (s *Store) UpdateProjectError(ctx context.Context, projectID, userID, message string) error {
	return s.updateProjectError(ctx, projectID, userID, publicProjectErrorMessage(message), internalProjectErrorMessage(message))
}

func (s *Store) UpdateProjectErrorFromError(ctx context.Context, projectID, userID string, err error) error {
	return s.updateProjectError(ctx, projectID, userID, publicProjectErrorMessageFromError(err), internalProjectErrorMessageFromError(err))
}

func (s *Store) UpdateProjectProvisioningRetryError(ctx context.Context, projectID, userID, status string, err error) error {
	status = strings.TrimSpace(status)
	if status == "" {
		status = "creating"
	}
	result, execErr := s.db.ExecContext(ctx, `
		UPDATE projects
		SET status = ?, internal_error_message = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND status != 'deleting'
	`, status, internalProjectErrorMessageFromError(err), nowString(), projectID, userID)
	if execErr != nil {
		return execErr
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) updateProjectError(ctx context.Context, projectID, userID, message, internalMessage string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET status = 'error', error_message = ?, internal_error_message = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND status != 'deleting'
	`, message, internalMessage, nowString(), projectID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func internalProjectErrorMessageFromError(err error) string {
	if err == nil {
		return ""
	}
	return internalProjectErrorMessage(err.Error())
}

func internalProjectErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	const maxInternalProjectErrorMessage = 2000
	if len(message) > maxInternalProjectErrorMessage {
		return message[:maxInternalProjectErrorMessage]
	}
	return message
}

type projectPublicErrorKind interface {
	PublicProjectErrorKind() string
}

func publicProjectErrorMessageFromError(err error) string {
	if err == nil {
		return publicProjectErrorMessage("")
	}
	var classified projectPublicErrorKind
	if errors.As(err, &classified) {
		switch classified.PublicProjectErrorKind() {
		case "configuration":
			return "Workspace settings are incomplete. Ask an admin to review the configuration, then create a new project."
		case "runtime_billing":
			return "The workspace runtime is not funded. Ask an admin to fund the linked Fibe workspace, then retry starting the project."
		case "timeout":
			return "The canvas took too long to start. Try creating a new project."
		}
	}
	return publicProjectErrorMessage(err.Error())
}

func publicProjectErrorMessage(message string) string {
	raw := strings.ToLower(strings.TrimSpace(message))
	switch {
	case raw == "":
		return "The canvas could not start. Ask an admin to check workspace settings, then create a new project."
	case strings.Contains(raw, "linked fibe playground is in an error state"):
		return "The linked Fibe playground is in an error state. Check it in Fibe, then restart the project playground from the project menu."
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
		WHERE id = ? AND user_id = ? AND (status != 'deleting' OR ? = 'deleting')
	`, status, nowString(), projectID, userID, status)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateProjectAssignment(ctx context.Context, projectID, userID, agentID, marqueeID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET agent_id = ?, marquee_id = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND status NOT IN ('deleting', 'archived')
	`, strings.TrimSpace(agentID), strings.TrimSpace(marqueeID), nowString(), projectID, userID)
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

func (s *Store) UpdateProjectSelectedService(ctx context.Context, projectID, userID, serviceName string) error {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return sql.ErrNoRows
	}
	var serviceURL string
	row := s.db.QueryRowContext(ctx, `
		SELECT project_services.url
		FROM project_services
		JOIN projects ON projects.id = project_services.project_id
		WHERE project_services.project_id = ? AND projects.user_id = ? AND project_services.name = ?
	`, projectID, userID, serviceName)
	if err := row.Scan(&serviceURL); err != nil {
		return err
	}
	now := nowString()
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET selected_service_name = ?, preview_url = ?, updated_at = ?
		WHERE id = ? AND user_id = ?
	`, serviceName, serviceURL, now, projectID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpsertProjectDomain(ctx context.Context, userID, projectID, domain, target string) error {
	domain = strings.TrimSpace(strings.ToLower(domain))
	target = strings.TrimSpace(target)
	now := nowString()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO project_domains(project_id, user_id, domain, target, status, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET domain = excluded.domain, target = excluded.target, status = excluded.status, updated_at = excluded.updated_at
	`, projectID, userID, domain, target, ProjectDomainStatusPendingDNS, now, now)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateProjectDomainStatus(ctx context.Context, userID, projectID, status string) error {
	status = strings.TrimSpace(status)
	if status == "" {
		return errors.New("project domain status is required")
	}
	now := nowString()
	result, err := s.db.ExecContext(ctx, `
		UPDATE project_domains
		SET status = ?, updated_at = ?
		WHERE project_id = ? AND user_id = ?
	`, status, now, projectID, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) PendingProjectDomains(ctx context.Context, limit int) ([]ProjectDomain, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT project_id, user_id, domain, target, status, updated_at
		FROM project_domains
		WHERE status = ?
		ORDER BY updated_at ASC
		LIMIT ?
	`, ProjectDomainStatusPendingDNS, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProjectDomain{}
	for rows.Next() {
		var projectDomain ProjectDomain
		if err := rows.Scan(&projectDomain.ProjectID, &projectDomain.UserID, &projectDomain.Domain, &projectDomain.Target, &projectDomain.Status, &projectDomain.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, projectDomain)
	}
	return out, rows.Err()
}

func (s *Store) DeleteProjectDomain(ctx context.Context, userID, projectID string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM project_domains
		WHERE project_id = ? AND user_id = ?
	`, projectID, userID)
	return err
}

func (s *Store) TouchProjectPlaygroundUsage(ctx context.Context, projectID, userID string) error {
	now := nowString()
	result, err := s.db.ExecContext(ctx, `
		UPDATE projects
		SET playground_last_used_at = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND status != 'deleting'
	`, now, now, projectID, userID)
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

func (s *Store) SaveProjectProvisioningSnapshot(ctx context.Context, project *Project, status string) error {
	if project == nil {
		return errors.New("project is required")
	}
	if status == "" {
		status = project.Status
	}
	now := nowString()
	lastUsedAt := strings.TrimSpace(project.PlaygroundLastUsedAt)
	if status == "ready" && lastUsedAt == "" {
		lastUsedAt = now
		project.PlaygroundLastUsedAt = lastUsedAt
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE projects
		SET agent_id = ?, marquee_id = ?, playground_id = ?, playground_name = ?, playspec_id = ?, prop_id = ?, repo_url = ?, preview_url = ?, selected_service_name = ?, status = ?, error_message = '', internal_error_message = '', cleanup_last_error = '',
			playground_last_used_at = CASE WHEN playground_last_used_at = '' AND ? != '' THEN ? ELSE playground_last_used_at END,
			updated_at = ?
		WHERE id = ? AND user_id = ? AND status != 'deleting'
	`, project.AgentID, project.MarqueeID, project.PlaygroundID, project.PlaygroundName, project.PlayspecID, project.PropID, project.RepoURL, project.PreviewURL, project.SelectedService, status, lastUsedAt, lastUsedAt, now, project.ID, project.UserID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	if err := replaceProjectResourcesTx(ctx, tx, project.ID, project.Repositories, project.Services); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ProjectForUser(ctx context.Context, userID, projectID string) (*Project, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playground_name, playspec_id, prop_id, repo_url, preview_url, selected_service_name, status, error_message, provisioning_lock_until, cleanup_last_error, playground_last_used_at, created_at, updated_at
		FROM projects
		WHERE id = ? AND user_id = ?
	`, projectID, userID)
	project, err := scanProject(row)
	if err != nil {
		return nil, err
	}
	return project, s.attachProjectResources(ctx, project)
}

func (s *Store) AllProjectsForUser(ctx context.Context, userID string) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playground_name, playspec_id, prop_id, repo_url, preview_url, selected_service_name, status, error_message, provisioning_lock_until, cleanup_last_error, playground_last_used_at, created_at, updated_at
		FROM projects
		WHERE user_id = ?
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	var out []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, *project)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := s.attachProjectResourcesForProjects(ctx, out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Project{}
	}
	return out, nil
}

func (s *Store) ProjectsForUser(ctx context.Context, userID string) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playground_name, playspec_id, prop_id, repo_url, preview_url, selected_service_name, status, error_message, provisioning_lock_until, cleanup_last_error, playground_last_used_at, created_at, updated_at
		FROM projects
		WHERE user_id = ? AND status != 'deleting'
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	var out []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, *project)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := s.attachProjectResourcesForProjects(ctx, out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Project{}
	}
	return out, nil
}

func (s *Store) DeletingProjects(ctx context.Context, limit int) ([]Project, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playground_name, playspec_id, prop_id, repo_url, preview_url, selected_service_name, status, error_message, provisioning_lock_until, cleanup_last_error, playground_last_used_at, created_at, updated_at
		FROM projects
		WHERE status = 'deleting'
		ORDER BY updated_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	var out []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, *project)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := s.attachProjectResourcesForProjects(ctx, out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Project{}
	}
	return out, nil
}

func (s *Store) StoppedProductionProjects(ctx context.Context, limit int) ([]Project, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playground_name, playspec_id, prop_id, repo_url, preview_url, selected_service_name, status, error_message, provisioning_lock_until, cleanup_last_error, playground_last_used_at, created_at, updated_at
		FROM projects
		WHERE status = 'stopped' AND TRIM(playground_id) != ''
			AND EXISTS (
				SELECT 1
				FROM project_production_grants
				WHERE project_production_grants.project_id = projects.id
					AND project_production_grants.expires_at > ?
			)
		ORDER BY updated_at ASC
		LIMIT ?
	`, nowString(), limit)
	if err != nil {
		return nil, err
	}
	var out []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, *project)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := s.attachProjectResourcesForProjects(ctx, out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Project{}
	}
	return out, nil
}

func (s *Store) IdleProjectsForPlaygroundStop(ctx context.Context, cutoff time.Time, limit int) ([]Project, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	cutoffString := cutoff.UTC().Format(time.RFC3339Nano)
	rows, err := s.db.QueryContext(ctx, `
		SELECT projects.id, projects.user_id, projects.title, projects.conversation_id, projects.agent_id, projects.marquee_id,
			projects.playground_id, projects.playground_name, projects.playspec_id, projects.prop_id, projects.repo_url, projects.preview_url,
			projects.selected_service_name, projects.status, projects.error_message, projects.provisioning_lock_until, projects.cleanup_last_error, projects.playground_last_used_at, projects.created_at, projects.updated_at
		FROM projects
		WHERE projects.status = 'ready' AND TRIM(projects.playground_id) != '' AND TRIM(projects.playground_last_used_at) != '' AND projects.playground_last_used_at < ?
			AND NOT EXISTS (
				SELECT 1
				FROM project_production_grants
				WHERE project_production_grants.project_id = projects.id
					AND project_production_grants.expires_at > ?
			)
		ORDER BY projects.playground_last_used_at ASC
		LIMIT ?
	`, cutoffString, nowString(), limit)
	if err != nil {
		return nil, err
	}
	var out []Project
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.UserID, &project.Title, &project.ConversationID, &project.AgentID, &project.MarqueeID, &project.PlaygroundID, &project.PlaygroundName, &project.PlayspecID, &project.PropID, &project.RepoURL, &project.PreviewURL, &project.SelectedService, &project.Status, &project.ErrorMessage, &project.ProvisioningLockUntil, &project.CleanupLastError, &project.PlaygroundLastUsedAt, &project.CreatedAt, &project.UpdatedAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		project.RefreshComputedFields()
		out = append(out, project)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := s.attachProjectResourcesForProjects(ctx, out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Project{}
	}
	return out, nil
}

func (s *Store) ProjectIdleForPlaygroundStop(ctx context.Context, projectID string, cutoff time.Time) (bool, string, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT projects.status, projects.playground_id, projects.playground_last_used_at,
			COALESCE((SELECT MAX(project_production_grants.expires_at)
				FROM project_production_grants
				WHERE project_production_grants.project_id = projects.id
					AND project_production_grants.expires_at > ?), '')
		FROM projects
		WHERE projects.id = ?
	`, nowString(), projectID)
	var status, playgroundID, lastUsedAt, productionExpiresAt string
	if err := row.Scan(&status, &playgroundID, &lastUsedAt, &productionExpiresAt); err != nil {
		return false, "", err
	}
	if status != "ready" || strings.TrimSpace(playgroundID) == "" {
		return false, "", nil
	}
	if strings.TrimSpace(productionExpiresAt) != "" {
		return false, "production project active until " + productionExpiresAt, nil
	}
	lastUsedAt = strings.TrimSpace(lastUsedAt)
	if lastUsedAt == "" {
		return false, "missing playground_last_used_at", nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, lastUsedAt)
	if err != nil {
		return false, "invalid playground_last_used_at", nil
	}
	return parsed.Before(cutoff.UTC()), "", nil
}

func scanProject(scanner interface{ Scan(...any) error }) (*Project, error) {
	var project Project
	if err := scanner.Scan(&project.ID, &project.UserID, &project.Title, &project.ConversationID, &project.AgentID, &project.MarqueeID, &project.PlaygroundID, &project.PlaygroundName, &project.PlayspecID, &project.PropID, &project.RepoURL, &project.PreviewURL, &project.SelectedService, &project.Status, &project.ErrorMessage, &project.ProvisioningLockUntil, &project.CleanupLastError, &project.PlaygroundLastUsedAt, &project.CreatedAt, &project.UpdatedAt); err != nil {
		return nil, err
	}
	project.RefreshComputedFields()
	return &project, nil
}

func (s *Store) ReplaceProjectResources(ctx context.Context, projectID string, repositories []ProjectRepository, services []ProjectService) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := replaceProjectResourcesTx(ctx, tx, projectID, repositories, services); err != nil {
		return err
	}
	return tx.Commit()
}

func replaceProjectResourcesTx(ctx context.Context, tx *sql.Tx, projectID string, repositories []ProjectRepository, services []ProjectService) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_repositories WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_services WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	now := nowString()
	for _, repository := range repositories {
		if repository.ID == "" {
			repository.ID = uuid.NewString()
		}
		if repository.CreatedAt == "" {
			repository.CreatedAt = now
		}
		serviceNames, err := json.Marshal(repository.ServiceNames)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO project_repositories(id, project_id, role, prop_id, repo_url, source_repo_url, provider, service_names, created_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, repository.ID, projectID, repository.Role, repository.PropID, repository.RepoURL, repository.SourceRepoURL, repository.Provider, string(serviceNames), repository.CreatedAt); err != nil {
			return err
		}
	}
	for _, service := range services {
		if service.ID == "" {
			service.ID = uuid.NewString()
		}
		if service.CreatedAt == "" {
			service.CreatedAt = now
		}
		authRequired := 0
		if service.AuthRequired {
			authRequired = 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO project_services(id, project_id, name, url, type, visibility, auth_required, created_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		`, service.ID, projectID, service.Name, service.URL, service.Type, service.Visibility, authRequired, service.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) attachProjectResources(ctx context.Context, project *Project) error {
	if project == nil {
		return nil
	}
	repositories, err := s.ProjectRepositories(ctx, project.ID)
	if err != nil {
		return err
	}
	services, err := s.ProjectServices(ctx, project.ID)
	if err != nil {
		return err
	}
	project.Repositories = repositories
	project.Services = services
	if project.SelectedService == "" && len(services) > 0 {
		project.SelectedService = services[0].Name
	}
	if project.PreviewURL == "" && len(services) > 0 {
		project.PreviewURL = services[0].URL
	}
	expiresAt, err := s.ActiveProjectProduction(ctx, project.UserID, project.ID)
	if err != nil {
		return err
	}
	project.ProductionExpiresAt = expiresAt
	if err := s.attachProjectDomain(ctx, project); err != nil {
		return err
	}
	project.RefreshComputedFields()
	return nil
}

func (s *Store) attachProjectDomain(ctx context.Context, project *Project) error {
	row := s.db.QueryRowContext(ctx, `
		SELECT domain, target, status, updated_at
		FROM project_domains
		WHERE project_id = ? AND user_id = ?
	`, project.ID, project.UserID)
	var domain, target, status, updatedAt string
	if err := row.Scan(&domain, &target, &status, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	project.CustomDomain = domain
	project.CustomDomainStatus = status
	project.CustomDomainTarget = projectDomainTarget(project)
	if project.CustomDomainTarget == "" {
		project.CustomDomainTarget = target
	}
	project.CustomDomainUpdatedAt = updatedAt
	return nil
}

func (s *Store) attachProjectResourcesForProjects(ctx context.Context, projects []Project) error {
	for i := range projects {
		if err := s.attachProjectResources(ctx, &projects[i]); err != nil {
			return err
		}
	}
	return nil
}

func projectDomainTarget(project *Project) string {
	if project == nil {
		return ""
	}
	selected := strings.TrimSpace(project.SelectedService)
	for _, service := range project.Services {
		if selected != "" && service.Name != selected {
			continue
		}
		if target := urlHost(service.URL); target != "" {
			return target
		}
	}
	if target := urlHost(project.PreviewURL); target != "" {
		return target
	}
	for _, service := range project.Services {
		if target := urlHost(service.URL); target != "" {
			return target
		}
	}
	return ""
}

func urlHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Host != "" {
		return parsed.Hostname()
	}
	rawURL = strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://")
	host := strings.Split(rawURL, "/")[0]
	if withoutPort, _, err := net.SplitHostPort(host); err == nil && withoutPort != "" {
		return strings.Trim(withoutPort, "[]")
	}
	return host
}

func (s *Store) ProjectRepositories(ctx context.Context, projectID string) ([]ProjectRepository, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, role, prop_id, repo_url, source_repo_url, provider, service_names, created_at
		FROM project_repositories
		WHERE project_id = ?
		ORDER BY created_at ASC, role ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProjectRepository{}
	for rows.Next() {
		var repository ProjectRepository
		var serviceNames string
		if err := rows.Scan(&repository.ID, &repository.ProjectID, &repository.Role, &repository.PropID, &repository.RepoURL, &repository.SourceRepoURL, &repository.Provider, &serviceNames, &repository.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(serviceNames), &repository.ServiceNames)
		if repository.ServiceNames == nil {
			repository.ServiceNames = []string{}
		}
		out = append(out, repository)
	}
	return out, rows.Err()
}

func (s *Store) ProjectServices(ctx context.Context, projectID string) ([]ProjectService, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, name, url, type, visibility, auth_required, created_at
		FROM project_services
		WHERE project_id = ?
		ORDER BY CASE name WHEN 'app' THEN 0 WHEN 'frontend' THEN 1 WHEN 'web' THEN 2 WHEN 'admin' THEN 3 ELSE 10 END, name ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProjectService{}
	for rows.Next() {
		var service ProjectService
		var authRequired int
		if err := rows.Scan(&service.ID, &service.ProjectID, &service.Name, &service.URL, &service.Type, &service.Visibility, &authRequired, &service.CreatedAt); err != nil {
			return nil, err
		}
		service.AuthRequired = authRequired != 0
		out = append(out, service)
	}
	return out, rows.Err()
}

func (s *Store) UsersWithProjects(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT users.id, users.email, users.name, users.avatar_url, users.access_status, users.access_note, users.created_at
		FROM users
			JOIN projects ON projects.user_id = users.id AND projects.status NOT IN ('deleting', 'archived')
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
			projects.playground_id, projects.playground_name, projects.playspec_id, projects.prop_id, projects.repo_url, projects.preview_url,
			projects.selected_service_name, projects.status, projects.error_message, projects.provisioning_lock_until, projects.cleanup_last_error, projects.playground_last_used_at, projects.created_at, projects.updated_at,
			COALESCE(MAX(messages.created_at), projects.updated_at) AS last_activity_at
		FROM projects
		LEFT JOIN messages ON messages.project_id = projects.id AND messages.role = 'user'
			WHERE projects.user_id = ? AND projects.status NOT IN ('deleting', 'archived')
		GROUP BY projects.id
		ORDER BY last_activity_at DESC, projects.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	var all []Project
	for rows.Next() {
		var project Project
		var lastActivity string
		if err := rows.Scan(&project.ID, &project.UserID, &project.Title, &project.ConversationID, &project.AgentID, &project.MarqueeID, &project.PlaygroundID, &project.PlaygroundName, &project.PlayspecID, &project.PropID, &project.RepoURL, &project.PreviewURL, &project.SelectedService, &project.Status, &project.ErrorMessage, &project.ProvisioningLockUntil, &project.CleanupLastError, &project.PlaygroundLastUsedAt, &project.CreatedAt, &project.UpdatedAt, &lastActivity); err != nil {
			_ = rows.Close()
			return nil, err
		}
		project.RefreshComputedFields()
		all = append(all, project)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := s.attachProjectResourcesForProjects(ctx, all); err != nil {
		return nil, err
	}
	if len(all) <= limit {
		return []Project{}, nil
	}
	return all[limit:], nil
}

func (s *Store) ProjectsForAssignment(ctx context.Context, agentID, marqueeID string) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, title, conversation_id, agent_id, marquee_id, playground_id, playground_name, playspec_id, prop_id, repo_url, preview_url, selected_service_name, status, error_message, provisioning_lock_until, cleanup_last_error, playground_last_used_at, created_at, updated_at
		FROM projects
		WHERE agent_id = ? AND marquee_id = ? AND status != 'deleting'
		ORDER BY updated_at DESC
	`, strings.TrimSpace(agentID), strings.TrimSpace(marqueeID))
	if err != nil {
		return nil, err
	}
	var out []Project
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, *project)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := s.attachProjectResourcesForProjects(ctx, out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Project{}
	}
	return out, nil
}
