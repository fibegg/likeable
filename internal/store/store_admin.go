package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (s *Store) AdminUsers(ctx context.Context, filters AdminUserFilters) ([]AdminUserSummary, int, error) {
	if filters.Page <= 0 {
		filters.Page = 1
	}
	if filters.PerPage <= 0 || filters.PerPage > 100 {
		filters.PerPage = 25
	}
	where, args := adminUserWhere(filters)
	usageWindowStart := filters.UsageWindowStart
	if usageWindowStart.IsZero() {
		usageWindowStart = time.Now().UTC().Add(-5 * time.Hour)
	}
	usageWindowEnd := filters.UsageWindowEnd
	if usageWindowEnd.IsZero() || !usageWindowEnd.After(usageWindowStart) {
		usageWindowEnd = usageWindowStart.UTC().Add(5 * time.Hour)
	}
	cte := `
		WITH stats AS (
			SELECT users.id AS user_id,
				COUNT(DISTINCT projects.id) AS project_count,
				MAX(projects.updated_at) AS last_project_at
				FROM users
				LEFT JOIN projects ON projects.user_id = users.id AND projects.status NOT IN ('deleting', 'archived')
			GROUP BY users.id
		),
		work AS (
			SELECT user_id, SUM(elapsed_ms) AS lifetime_work_ms, MAX(completed_at) AS last_work_at
			FROM project_work_sessions
			WHERE billed_at != ''
			GROUP BY user_id
		),
		github AS (
			SELECT user_id, 1 AS github_connected
			FROM social_connections
			WHERE provider = 'github'
			GROUP BY user_id
		),
		payments_total AS (
			SELECT user_id, SUM(amount_cents) AS paid_total_cents, MAX(currency) AS paid_currency
			FROM payments
			WHERE status IN ('paid', 'complete', 'completed', 'succeeded')
			GROUP BY user_id
		),
			hour_credits AS (
				SELECT user_id, SUM(delta_ms) AS paid_hour_balance_ms
				FROM hour_credit_ledger
				GROUP BY user_id
			),
			project_quota AS (
				SELECT user_id, SUM(slots) AS paid_project_slots, MIN(expires_at) AS project_slots_expire
				FROM project_quota_ledger
				WHERE expires_at > '` + nowString() + `'
				GROUP BY user_id
			)
		`
	from := `
		FROM users
		LEFT JOIN stats ON stats.user_id = users.id
		LEFT JOIN github ON github.user_id = users.id
			LEFT JOIN subscriptions ON subscriptions.user_id = users.id
			LEFT JOIN payments_total ON payments_total.user_id = users.id
			LEFT JOIN work ON work.user_id = users.id
			LEFT JOIN hour_credits ON hour_credits.user_id = users.id
			LEFT JOIN project_quota ON project_quota.user_id = users.id
		`
	var total int
	countQuery := cte + ` SELECT COUNT(*) ` + from + where
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filters.PerPage, (filters.Page-1)*filters.PerPage)
	rows, err := s.db.QueryContext(ctx, cte+`
		SELECT users.id, users.email, users.name, users.avatar_url, users.access_status, users.access_note, users.created_at,
				COALESCE(stats.project_count, 0),
				COALESCE(project_quota.paid_project_slots, 0),
				COALESCE(project_quota.project_slots_expire, ''),
				COALESCE(work.lifetime_work_ms, 0),
			0,
			COALESCE(hour_credits.paid_hour_balance_ms, 0),
			COALESCE(github.github_connected, 0),
			COALESCE(subscriptions.status, ''),
			COALESCE(payments_total.paid_total_cents, 0),
			COALESCE(payments_total.paid_currency, ''),
			COALESCE(work.last_work_at, ''),
			COALESCE(stats.last_project_at, ''),
			COALESCE((SELECT id FROM user_notices WHERE user_id = users.id AND unsent_at = '' ORDER BY created_at DESC LIMIT 1), ''),
			COALESCE((SELECT sender FROM user_notices WHERE user_id = users.id AND unsent_at = '' ORDER BY created_at DESC LIMIT 1), ''),
			COALESCE((SELECT severity FROM user_notices WHERE user_id = users.id AND unsent_at = '' ORDER BY created_at DESC LIMIT 1), ''),
			COALESCE((SELECT body FROM user_notices WHERE user_id = users.id AND unsent_at = '' ORDER BY created_at DESC LIMIT 1), ''),
			COALESCE((SELECT read_at FROM user_notices WHERE user_id = users.id AND unsent_at = '' ORDER BY created_at DESC LIMIT 1), ''),
			COALESCE((SELECT dismissed_at FROM user_notices WHERE user_id = users.id AND unsent_at = '' ORDER BY created_at DESC LIMIT 1), ''),
			COALESCE((SELECT created_at FROM user_notices WHERE user_id = users.id AND unsent_at = '' ORDER BY created_at DESC LIMIT 1), '')
	`+from+where+adminUserOrder(filters.Sort)+` LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	var out []AdminUserSummary
	for rows.Next() {
		var summary AdminUserSummary
		var notice UserNotice
		var latestNoticeID string
		if err := rows.Scan(
			&summary.User.ID, &summary.User.Email, &summary.User.Name, &summary.User.AvatarURL, &summary.User.AccessStatus, &summary.User.AccessNote, &summary.User.CreatedAt,
			&summary.ProjectCount, &summary.PaidProjectSlots, &summary.ProjectSlotsExpire, &summary.LifetimeWorkMs, &summary.WindowWorkMs, &summary.PaidHourBalanceMs, &summary.GithubConnected, &summary.SubscriptionStatus, &summary.PaidTotalCents, &summary.PaidCurrency,
			&summary.LastMessageAt, &summary.LastProjectAt,
			&latestNoticeID, &notice.Sender, &notice.Severity, &notice.Body, &notice.ReadAt, &notice.DismissedAt, &notice.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if summary.User.AccessStatus == "" {
			summary.User.AccessStatus = "active"
		}
		if latestNoticeID != "" {
			notice.ID = latestNoticeID
			notice.UserID = summary.User.ID
			summary.LatestNotice = &notice
		}
		out = append(out, summary)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	now := time.Now().UTC()
	for i := range out {
		if workMs, err := s.UserWorkMsBetween(ctx, out[i].User.ID, usageWindowStart, usageWindowEnd, now); err == nil {
			out[i].WindowWorkMs = workMs
		}
		if workMs, err := s.UserLifetimeWorkMs(ctx, out[i].User.ID, now); err == nil {
			out[i].LifetimeWorkMs = workMs
		}
	}
	if out == nil {
		out = []AdminUserSummary{}
	}
	if err := s.attachAdminUserAgentPairs(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *Store) attachAdminUserAgentPairs(ctx context.Context, users []AdminUserSummary) error {
	if len(users) == 0 {
		return nil
	}
	args := make([]any, 0, len(users))
	placeholders := make([]string, 0, len(users))
	indexByUserID := make(map[string]int, len(users))
	for i := range users {
		userID := strings.TrimSpace(users[i].User.ID)
		if userID == "" {
			continue
		}
		indexByUserID[userID] = i
		args = append(args, userID)
		placeholders = append(placeholders, "?")
	}
	if len(args) == 0 {
		return nil
	}
	query := `
		SELECT user_id, agent_id, marquee_id, COUNT(*)
		FROM projects
		WHERE user_id IN (` + strings.Join(placeholders, ",") + `)
			AND status NOT IN ('deleting', 'archived')
			AND TRIM(agent_id) != ''
			AND TRIM(marquee_id) != ''
		GROUP BY user_id, agent_id, marquee_id
		ORDER BY COUNT(*) DESC, agent_id ASC, marquee_id ASC
	`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID string
		var pair AgentAssignmentSummary
		if err := rows.Scan(&userID, &pair.AgentID, &pair.ServerID, &pair.ProjectCount); err != nil {
			return err
		}
		if index, ok := indexByUserID[userID]; ok {
			users[index].AgentPairs = append(users[index].AgentPairs, pair)
		}
	}
	return rows.Err()
}

func adminUserWhere(filters AdminUserFilters) (string, []any) {
	var where []string
	var args []any
	if q := strings.TrimSpace(filters.Query); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		where = append(where, "(lower(users.email) LIKE ? OR lower(users.name) LIKE ? OR users.id = ?)")
		args = append(args, like, like, q)
	}
	switch strings.ToLower(strings.TrimSpace(filters.Status)) {
	case "active", "restricted":
		where = append(where, "users.access_status = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(filters.Status)))
	}
	switch strings.ToLower(strings.TrimSpace(filters.Github)) {
	case "connected":
		where = append(where, "COALESCE(github.github_connected, 0) = 1")
	case "missing":
		where = append(where, "COALESCE(github.github_connected, 0) = 0")
	}
	switch strings.ToLower(strings.TrimSpace(filters.Billing)) {
	case "paid":
		where = append(where, "COALESCE(payments_total.paid_total_cents, 0) > 0")
	case "unpaid":
		where = append(where, "COALESCE(payments_total.paid_total_cents, 0) = 0")
	case "subscribed":
		where = append(where, "subscriptions.status IN ('active', 'trialing')")
	}
	if len(where) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(where, " AND "), args
}

func (s *Store) AgentPoolStats(ctx context.Context) ([]AgentPoolStat, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT projects.agent_id,
			projects.marquee_id,
			COUNT(DISTINCT projects.id) AS project_count,
			COUNT(DISTINCT CASE WHEN projects.status != 'archived' THEN projects.id END) AS active_project_count,
			SUM(CASE WHEN projects.status = 'archived' THEN 1 ELSE 0 END) AS archived_count,
			COUNT(DISTINCT CASE WHEN project_archives.id IS NOT NULL THEN projects.id END) AS ready_archive_count
		FROM projects
		LEFT JOIN project_archives ON project_archives.project_id = projects.id
			AND project_archives.status = 'ready'
			AND project_archives.expires_at > ?
		WHERE projects.status != 'deleting'
			AND TRIM(projects.agent_id) != ''
			AND TRIM(projects.marquee_id) != ''
		GROUP BY projects.agent_id, projects.marquee_id
	`, nowString())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AgentPoolStat
	for rows.Next() {
		var stat AgentPoolStat
		if err := rows.Scan(&stat.AgentID, &stat.ServerID, &stat.ProjectCount, &stat.ActiveProjectCount, &stat.ArchivedCount, &stat.ReadyArchiveCount); err != nil {
			return nil, err
		}
		out = append(out, stat)
	}
	if out == nil {
		out = []AgentPoolStat{}
	}
	return out, rows.Err()
}

func adminUserOrder(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "email_asc":
		return " ORDER BY users.email ASC"
	case "hours_desc":
		return " ORDER BY COALESCE(work.lifetime_work_ms, 0) DESC, users.created_at DESC"
	case "paid_desc":
		return " ORDER BY COALESCE(payments_total.paid_total_cents, 0) DESC, users.created_at DESC"
	case "projects_desc":
		return " ORDER BY COALESCE(stats.project_count, 0) DESC, users.created_at DESC"
	default:
		return " ORDER BY users.created_at DESC"
	}
}

func (s *Store) AdminUserDetail(ctx context.Context, userID string, freeLimitMs int64, usageWindowStart, usageWindowEnd time.Time) (*AdminUserDetail, error) {
	users, _, err := s.AdminUsers(ctx, AdminUserFilters{Query: userID, Page: 1, PerPage: 1, UsageWindowStart: usageWindowStart, UsageWindowEnd: usageWindowEnd})
	if err != nil {
		return nil, err
	}
	var summary *AdminUserSummary
	for i := range users {
		if users[i].User.ID == userID {
			summary = &users[i]
			break
		}
	}
	if summary == nil {
		if _, err := s.UserByID(ctx, userID); err != nil {
			return nil, err
		}
		users, _, err = s.AdminUsers(ctx, AdminUserFilters{Page: 1, PerPage: 100, UsageWindowStart: usageWindowStart, UsageWindowEnd: usageWindowEnd})
		if err != nil {
			return nil, err
		}
		for i := range users {
			if users[i].User.ID == userID {
				summary = &users[i]
				break
			}
		}
	}
	if summary == nil {
		return nil, sql.ErrNoRows
	}
	summary.FreeHourLimitMs = freeLimitMs
	projects, err := s.AdminProjectsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	notices, err := s.NoticesForUser(ctx, userID, 50)
	if err != nil {
		return nil, err
	}
	return &AdminUserDetail{Summary: *summary, Projects: projects, Notices: notices}, nil
}

func (s *Store) AdminProjectsForUser(ctx context.Context, userID string) ([]AdminProjectSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT projects.id, projects.user_id, projects.title, projects.conversation_id, projects.agent_id, projects.marquee_id, projects.playground_id, projects.playground_name, projects.playspec_id, projects.prop_id, projects.repo_url, projects.preview_url, projects.selected_service_name, projects.status, projects.error_message, projects.provisioning_lock_until, projects.cleanup_last_error, projects.playground_last_used_at, projects.created_at, projects.updated_at,
			COALESCE((SELECT SUM(elapsed_ms) FROM project_work_sessions WHERE project_work_sessions.project_id = projects.id AND billed_at != ''), 0) AS work_ms
		FROM projects
		WHERE projects.user_id = ? AND projects.status != 'deleting'
		GROUP BY projects.id
		ORDER BY projects.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	var out []AdminProjectSummary
	for rows.Next() {
		var summary AdminProjectSummary
		if err := rows.Scan(&summary.Project.ID, &summary.Project.UserID, &summary.Project.Title, &summary.Project.ConversationID, &summary.Project.AgentID, &summary.Project.MarqueeID, &summary.Project.PlaygroundID, &summary.Project.PlaygroundName, &summary.Project.PlayspecID, &summary.Project.PropID, &summary.Project.RepoURL, &summary.Project.PreviewURL, &summary.Project.SelectedService, &summary.Project.Status, &summary.Project.ErrorMessage, &summary.Project.ProvisioningLockUntil, &summary.Project.CleanupLastError, &summary.Project.PlaygroundLastUsedAt, &summary.Project.CreatedAt, &summary.Project.UpdatedAt, &summary.WorkMs); err != nil {
			_ = rows.Close()
			return nil, err
		}
		summary.Assignment = AgentAssignmentSummary{
			AgentID:  strings.TrimSpace(summary.Project.AgentID),
			ServerID: strings.TrimSpace(summary.Project.MarqueeID),
		}
		summary.Project.RefreshComputedFields()
		out = append(out, summary)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := s.attachProjectResources(ctx, &out[i].Project); err != nil {
			return nil, err
		}
	}
	if out == nil {
		out = []AdminProjectSummary{}
	}
	return out, nil
}
