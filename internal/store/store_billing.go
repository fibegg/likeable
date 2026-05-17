package store

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"strings"
	"time"
)

const hourCreditMs = int64(time.Hour / time.Millisecond)

type workInterval struct {
	start time.Time
	end   time.Time
}

func (s *Store) PaidMessageCreditBalance(ctx context.Context, userID string) (int, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(delta), 0)
		FROM message_credit_ledger
		WHERE user_id = ?
	`, userID)
	var balance int
	if err := row.Scan(&balance); err != nil {
		return 0, err
	}
	if balance < 0 {
		return 0, nil
	}
	return balance, nil
}

func (s *Store) GrantMessageCredits(ctx context.Context, userID, paymentID string, count int) (bool, error) {
	if count <= 0 {
		return false, nil
	}
	paymentID = strings.TrimSpace(paymentID)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if paymentID != "" {
		var existing int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM message_credit_ledger
			WHERE payment_id = ? AND reason = 'purchase'
		`, paymentID).Scan(&existing); err != nil {
			return false, err
		}
		if existing > 0 {
			return false, tx.Commit()
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO message_credit_ledger(id, user_id, delta, reason, payment_id, created_at)
		VALUES(?, ?, ?, 'purchase', ?, ?)
	`, uuid.NewString(), userID, count, paymentID, nowString())
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (s *Store) ConsumePaidMessageCredit(ctx context.Context, userID, messageID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var balance int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(delta), 0)
		FROM message_credit_ledger
		WHERE user_id = ?
	`, userID).Scan(&balance); err != nil {
		return err
	}
	if balance <= 0 {
		return errors.New("no paid message credits available")
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO message_credit_ledger(id, user_id, delta, reason, message_id, created_at)
		VALUES(?, ?, -1, 'message', ?, ?)
	`, uuid.NewString(), userID, strings.TrimSpace(messageID), nowString())
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PaidHourCreditBalance(ctx context.Context, userID string) (int64, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(delta_ms), 0)
		FROM hour_credit_ledger
		WHERE user_id = ?
	`, userID)
	var balance int64
	if err := row.Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

func (s *Store) GrantHourCredits(ctx context.Context, userID, paymentID string, hours int) (bool, error) {
	if hours <= 0 {
		return false, nil
	}
	paymentID = strings.TrimSpace(paymentID)
	if paymentID == "" {
		paymentID = uuid.NewString()
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO hour_credit_ledger(id, user_id, delta_ms, reason, payment_id, created_at)
		VALUES(?, ?, ?, 'purchase', ?, ?)
	`, uuid.NewString(), userID, int64(hours)*hourCreditMs, paymentID, nowString())
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (s *Store) StartProjectWorkSession(ctx context.Context, userID, projectID, sessionKey string, startedAt time.Time) error {
	sessionKey = strings.TrimSpace(sessionKey)
	if userID == "" || projectID == "" || sessionKey == "" {
		return nil
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	now := nowString()
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO project_work_sessions(project_id, user_id, session_key, started_at, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?)
	`, projectID, userID, sessionKey, startedAt.UTC().Format(time.RFC3339Nano), now, now)
	return err
}

func (s *Store) CompleteAndBillProjectWorkSession(ctx context.Context, userID, projectID, sessionKey string, completedAt time.Time, windowHours int, freeLimitMs int64) (bool, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	if userID == "" || projectID == "" || sessionKey == "" {
		return false, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	session, err := projectWorkSessionForUpdate(ctx, tx, userID, projectID, sessionKey)
	if errors.Is(err, sql.ErrNoRows) {
		return false, tx.Commit()
	}
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(session.BilledAt) != "" {
		return false, tx.Commit()
	}
	startedAt, ok := parseStoredTime(session.StartedAt)
	if !ok {
		startedAt = completedAt.UTC()
	}
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	completedAt = completedAt.UTC()
	if parsed, ok := parseStoredTime(session.CompletedAt); ok {
		completedAt = parsed
	}
	if completedAt.Before(startedAt) {
		completedAt = startedAt
	}
	elapsedMs := completedAt.Sub(startedAt).Milliseconds()
	if elapsedMs < 0 {
		elapsedMs = 0
	}
	freeMs, paidMs, err := txWorkBillingSplit(ctx, tx, userID, projectID, sessionKey, startedAt, completedAt, windowHours, freeLimitMs)
	if err != nil {
		return false, err
	}
	billedAt := nowString()
	_, err = tx.ExecContext(ctx, `
		UPDATE project_work_sessions
		SET completed_at = ?, elapsed_ms = ?, free_billed_ms = ?, paid_billed_ms = ?, billed_at = ?, updated_at = ?
		WHERE project_id = ? AND user_id = ? AND session_key = ? AND billed_at = ''
	`, completedAt.Format(time.RFC3339Nano), elapsedMs, freeMs, paidMs, billedAt, billedAt, projectID, userID, sessionKey)
	if err != nil {
		return false, err
	}
	if paidMs > 0 {
		_, err = tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO hour_credit_ledger(id, user_id, delta_ms, reason, work_session_key, created_at)
			VALUES(?, ?, ?, 'work_session', ?, ?)
		`, uuid.NewString(), userID, -paidMs, projectWorkLedgerKey(projectID, sessionKey), billedAt)
		if err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

func (s *Store) CompleteOpenProjectWorkSessions(ctx context.Context, userID, projectID string, completedAt time.Time, windowHours int, freeLimitMs int64) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT session_key
		FROM project_work_sessions
		WHERE user_id = ? AND project_id = ? AND completed_at = ''
	`, userID, projectID)
	if err != nil {
		return 0, err
	}
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			_ = rows.Close()
			return 0, err
		}
		keys = append(keys, key)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	completed := 0
	for _, key := range keys {
		billed, err := s.CompleteAndBillProjectWorkSession(ctx, userID, projectID, key, completedAt, windowHours, freeLimitMs)
		if err != nil {
			return completed, err
		}
		if billed {
			completed++
		}
	}
	return completed, nil
}

func (s *Store) UserWorkMsBetween(ctx context.Context, userID string, from, to, now time.Time) (int64, error) {
	if !to.After(from) {
		return 0, nil
	}
	intervals, err := s.userWorkIntervals(ctx, userID, from, to, now)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, interval := range intervals {
		total += overlapMs(interval.start, interval.end, from, to)
	}
	return total, nil
}

func (s *Store) UserLifetimeWorkMs(ctx context.Context, userID string, now time.Time) (int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT started_at, completed_at
		FROM project_work_sessions
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var total int64
	for rows.Next() {
		var startedRaw, completedRaw string
		if err := rows.Scan(&startedRaw, &completedRaw); err != nil {
			return 0, err
		}
		startedAt, ok := parseStoredTime(startedRaw)
		if !ok {
			continue
		}
		completedAt, ok := parseStoredTime(completedRaw)
		if !ok {
			completedAt = now.UTC()
		}
		if completedAt.After(startedAt) {
			total += completedAt.Sub(startedAt).Milliseconds()
		}
	}
	return total, rows.Err()
}

func projectWorkSessionForUpdate(ctx context.Context, tx *sql.Tx, userID, projectID, sessionKey string) (*ProjectWorkSession, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT project_id, user_id, session_key, started_at, completed_at, elapsed_ms, free_billed_ms, paid_billed_ms, billed_at, created_at, updated_at
		FROM project_work_sessions
		WHERE user_id = ? AND project_id = ? AND session_key = ?
	`, userID, projectID, sessionKey)
	var session ProjectWorkSession
	if err := row.Scan(&session.ProjectID, &session.UserID, &session.SessionKey, &session.StartedAt, &session.CompletedAt, &session.ElapsedMs, &session.FreeBilledMs, &session.PaidBilledMs, &session.BilledAt, &session.CreatedAt, &session.UpdatedAt); err != nil {
		return nil, err
	}
	return &session, nil
}

func txWorkBillingSplit(ctx context.Context, tx *sql.Tx, userID, projectID, sessionKey string, startedAt, completedAt time.Time, windowHours int, freeLimitMs int64) (int64, int64, error) {
	if !completedAt.After(startedAt) {
		return 0, 0, nil
	}
	if freeLimitMs < 0 {
		freeLimitMs = 0
	}
	window := time.Duration(windowHours) * time.Hour
	if window <= 0 || window > 24*time.Hour {
		window = 5 * time.Hour
	}
	var freeMs int64
	cursor := startedAt.UTC()
	for cursor.Before(completedAt) {
		windowStart, windowEnd := fixedUTCStoreWindow(cursor, window)
		segmentEnd := minTime(completedAt, windowEnd)
		segmentMs := segmentEnd.Sub(cursor).Milliseconds()
		if segmentMs <= 0 {
			break
		}
		previousMs, err := txUserBilledWorkMsBetween(ctx, tx, userID, projectID, sessionKey, windowStart, windowEnd)
		if err != nil {
			return 0, 0, err
		}
		remainingFreeMs := freeLimitMs - previousMs
		if remainingFreeMs < 0 {
			remainingFreeMs = 0
		}
		freeMs += minInt64(segmentMs, remainingFreeMs)
		cursor = segmentEnd
	}
	elapsedMs := completedAt.Sub(startedAt).Milliseconds()
	paidMs := elapsedMs - freeMs
	if paidMs < 0 {
		paidMs = 0
	}
	return freeMs, paidMs, nil
}

func txUserBilledWorkMsBetween(ctx context.Context, tx *sql.Tx, userID, excludedProjectID, excludedSessionKey string, from, to time.Time) (int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT started_at, completed_at
		FROM project_work_sessions
		WHERE user_id = ?
			AND billed_at != ''
			AND completed_at != ''
			AND NOT (project_id = ? AND session_key = ?)
	`, userID, excludedProjectID, excludedSessionKey)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var total int64
	for rows.Next() {
		var startedRaw, completedRaw string
		if err := rows.Scan(&startedRaw, &completedRaw); err != nil {
			return 0, err
		}
		startedAt, startOK := parseStoredTime(startedRaw)
		completedAt, completeOK := parseStoredTime(completedRaw)
		if !startOK || !completeOK {
			continue
		}
		total += overlapMs(startedAt, completedAt, from, to)
	}
	return total, rows.Err()
}

func (s *Store) userWorkIntervals(ctx context.Context, userID string, from, to, now time.Time) ([]workInterval, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT started_at, completed_at
		FROM project_work_sessions
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var intervals []workInterval
	for rows.Next() {
		var startedRaw, completedRaw string
		if err := rows.Scan(&startedRaw, &completedRaw); err != nil {
			return nil, err
		}
		startedAt, ok := parseStoredTime(startedRaw)
		if !ok {
			continue
		}
		completedAt, ok := parseStoredTime(completedRaw)
		if !ok {
			completedAt = now.UTC()
		}
		if completedAt.After(startedAt) && completedAt.After(from) && startedAt.Before(to) {
			intervals = append(intervals, workInterval{start: startedAt, end: completedAt})
		}
	}
	return intervals, rows.Err()
}

func projectWorkLedgerKey(projectID, sessionKey string) string {
	return strings.TrimSpace(projectID) + ":" + strings.TrimSpace(sessionKey)
}

func fixedUTCStoreWindow(now time.Time, interval time.Duration) (time.Time, time.Time) {
	if interval <= 0 {
		interval = 5 * time.Hour
	}
	if interval > 24*time.Hour {
		interval = 24 * time.Hour
	}
	now = now.UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	elapsed := now.Sub(dayStart)
	bucket := int64(elapsed / interval)
	start := dayStart.Add(time.Duration(bucket) * interval)
	end := start.Add(interval)
	nextDay := dayStart.Add(24 * time.Hour)
	if end.After(nextDay) {
		end = nextDay
	}
	return start, end
}

func parseStoredTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, raw)
	}
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func overlapMs(start, end, from, to time.Time) int64 {
	overlapStart := maxTime(start.UTC(), from.UTC())
	overlapEnd := minTime(end.UTC(), to.UTC())
	if !overlapEnd.After(overlapStart) {
		return 0
	}
	return overlapEnd.Sub(overlapStart).Milliseconds()
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}

func maxTime(left, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func (s *Store) GrantProjectQuota(ctx context.Context, userID, paymentID string, slots int, expiresAt time.Time) (bool, error) {
	if slots <= 0 {
		return false, nil
	}
	paymentID = strings.TrimSpace(paymentID)
	if paymentID == "" {
		paymentID = uuid.NewString()
	}
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(30 * 24 * time.Hour)
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO project_quota_ledger(id, user_id, payment_id, slots, expires_at, created_at)
		VALUES(?, ?, ?, ?, ?, ?)
	`, uuid.NewString(), userID, paymentID, slots, expiresAt.UTC().Format(time.RFC3339Nano), nowString())
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (s *Store) ActiveProjectQuota(ctx context.Context, userID string) (int, string, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(slots), 0), COALESCE(MIN(expires_at), '')
		FROM project_quota_ledger
		WHERE user_id = ? AND expires_at > ?
	`, userID, nowString())
	var slots int
	var nextExpiresAt string
	if err := row.Scan(&slots, &nextExpiresAt); err != nil {
		return 0, "", err
	}
	if slots < 0 {
		slots = 0
	}
	return slots, nextExpiresAt, nil
}

func (s *Store) UpsertSocialConnection(ctx context.Context, conn SocialConnection) error {
	if conn.ID == "" {
		conn.ID = uuid.NewString()
	}
	now := nowString()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO social_connections(id, user_id, provider, provider_user_id, access_token, scope, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, provider) DO UPDATE SET
			provider_user_id=excluded.provider_user_id,
			access_token=excluded.access_token,
			scope=excluded.scope,
			updated_at=excluded.updated_at
	`, conn.ID, conn.UserID, conn.Provider, conn.ProviderUserID, conn.AccessToken, conn.Scope, now, now)
	return err
}

func (s *Store) SocialConnection(ctx context.Context, userID, provider string) (*SocialConnection, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, provider, provider_user_id, access_token, scope
		FROM social_connections
		WHERE user_id = ? AND provider = ?
	`, userID, provider)
	var conn SocialConnection
	if err := row.Scan(&conn.ID, &conn.UserID, &conn.Provider, &conn.ProviderUserID, &conn.AccessToken, &conn.Scope); err != nil {
		return nil, err
	}
	return &conn, nil
}

func (s *Store) UpsertSubscription(ctx context.Context, sub Subscription) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subscriptions(user_id, status, stripe_customer_id, stripe_subscription_id, current_period_end, updated_at)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			status=excluded.status,
			stripe_customer_id=excluded.stripe_customer_id,
			stripe_subscription_id=excluded.stripe_subscription_id,
			current_period_end=excluded.current_period_end,
			updated_at=excluded.updated_at
	`, sub.UserID, sub.Status, sub.StripeCustomerID, sub.StripeSubscriptionID, sub.CurrentPeriodEnd.Format(time.RFC3339Nano), nowString())
	return err
}

func (s *Store) SubscriptionForUser(ctx context.Context, userID string) (*Subscription, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT user_id, status, stripe_customer_id, stripe_subscription_id, current_period_end
		FROM subscriptions WHERE user_id = ?
	`, userID)
	var sub Subscription
	var period string
	if err := row.Scan(&sub.UserID, &sub.Status, &sub.StripeCustomerID, &sub.StripeSubscriptionID, &period); err != nil {
		return nil, err
	}
	if period != "" {
		parsed, _ := time.Parse(time.RFC3339Nano, period)
		sub.CurrentPeriodEnd = parsed
	}
	return &sub, nil
}

func (s *Store) HasActiveSubscription(ctx context.Context, userID string) bool {
	sub, err := s.SubscriptionForUser(ctx, userID)
	if err != nil {
		return false
	}
	if sub.Status == "active" || sub.Status == "trialing" {
		return sub.CurrentPeriodEnd.IsZero() || sub.CurrentPeriodEnd.After(time.Now().UTC())
	}
	return false
}

func (s *Store) UpsertPayment(ctx context.Context, payment Payment) error {
	if payment.ID == "" {
		payment.ID = uuid.NewString()
	}
	if payment.CreatedAt == "" {
		payment.CreatedAt = nowString()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO payments(id, user_id, provider_payment_id, amount_cents, currency, status, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_payment_id) DO UPDATE SET
			amount_cents=excluded.amount_cents,
			currency=excluded.currency,
			status=excluded.status
	`, payment.ID, payment.UserID, payment.ProviderPaymentID, payment.AmountCents, strings.ToLower(payment.Currency), payment.Status, payment.CreatedAt)
	return err
}
