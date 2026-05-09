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
	dayStart := time.Now().UTC().Truncate(24 * time.Hour).Format(time.RFC3339Nano)
	cte := `
		WITH stats AS (
			SELECT users.id AS user_id,
				COUNT(DISTINCT projects.id) AS project_count,
				COUNT(messages.id) AS message_count,
				SUM(CASE WHEN messages.created_at >= '` + dayStart + `' THEN 1 ELSE 0 END) AS daily_message_count,
				MAX(messages.created_at) AS last_message_at,
				MAX(projects.updated_at) AS last_project_at
			FROM users
			LEFT JOIN projects ON projects.user_id = users.id AND projects.status != 'deleting'
			LEFT JOIN messages ON messages.project_id = projects.id AND messages.role = 'user'
			GROUP BY users.id
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
			credits AS (
				SELECT user_id, SUM(delta) AS paid_credit_balance
				FROM message_credit_ledger
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
			LEFT JOIN credits ON credits.user_id = users.id
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
				COALESCE(stats.message_count, 0),
			COALESCE(stats.daily_message_count, 0),
			COALESCE(credits.paid_credit_balance, 0),
			COALESCE(github.github_connected, 0),
			COALESCE(subscriptions.status, ''),
			COALESCE(payments_total.paid_total_cents, 0),
			COALESCE(payments_total.paid_currency, ''),
			COALESCE(stats.last_message_at, ''),
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
	defer rows.Close()
	var out []AdminUserSummary
	for rows.Next() {
		var summary AdminUserSummary
		var notice UserNotice
		var latestNoticeID string
		if err := rows.Scan(
			&summary.User.ID, &summary.User.Email, &summary.User.Name, &summary.User.AvatarURL, &summary.User.AccessStatus, &summary.User.AccessNote, &summary.User.CreatedAt,
			&summary.ProjectCount, &summary.PaidProjectSlots, &summary.ProjectSlotsExpire, &summary.MessageCount, &summary.DailyMessageCount, &summary.PaidCreditBalance, &summary.GithubConnected, &summary.SubscriptionStatus, &summary.PaidTotalCents, &summary.PaidCurrency,
			&summary.LastMessageAt, &summary.LastProjectAt,
			&latestNoticeID, &notice.Sender, &notice.Severity, &notice.Body, &notice.ReadAt, &notice.DismissedAt, &notice.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		summary.FreeMessageLimit = 0
		if summary.PaidCreditBalance < 0 {
			summary.PaidCreditBalance = 0
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
	if out == nil {
		out = []AdminUserSummary{}
	}
	return out, total, rows.Err()
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

func adminUserOrder(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "email_asc":
		return " ORDER BY users.email ASC"
	case "messages_desc":
		return " ORDER BY COALESCE(stats.message_count, 0) DESC, users.created_at DESC"
	case "paid_desc":
		return " ORDER BY COALESCE(payments_total.paid_total_cents, 0) DESC, users.created_at DESC"
	case "projects_desc":
		return " ORDER BY COALESCE(stats.project_count, 0) DESC, users.created_at DESC"
	default:
		return " ORDER BY users.created_at DESC"
	}
}

func (s *Store) AdminUserDetail(ctx context.Context, userID string, freeLimit int) (*AdminUserDetail, error) {
	users, _, err := s.AdminUsers(ctx, AdminUserFilters{Query: userID, Page: 1, PerPage: 1})
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
		users, _, err = s.AdminUsers(ctx, AdminUserFilters{Page: 1, PerPage: 100})
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
	summary.FreeMessageLimit = freeLimit
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
		SELECT projects.id, projects.user_id, projects.title, projects.conversation_id, projects.agent_id, projects.marquee_id, projects.playground_id, projects.playspec_id, projects.prop_id, projects.repo_url, projects.preview_url, projects.selected_service_name, projects.status, projects.error_message, projects.created_at, projects.updated_at,
			COUNT(messages.id) AS message_count
		FROM projects
		LEFT JOIN messages ON messages.project_id = projects.id AND messages.role = 'user'
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
		if err := rows.Scan(&summary.Project.ID, &summary.Project.UserID, &summary.Project.Title, &summary.Project.ConversationID, &summary.Project.AgentID, &summary.Project.MarqueeID, &summary.Project.PlaygroundID, &summary.Project.PlayspecID, &summary.Project.PropID, &summary.Project.RepoURL, &summary.Project.PreviewURL, &summary.Project.SelectedService, &summary.Project.Status, &summary.Project.ErrorMessage, &summary.Project.CreatedAt, &summary.Project.UpdatedAt, &summary.MessageCount); err != nil {
			_ = rows.Close()
			return nil, err
		}
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
