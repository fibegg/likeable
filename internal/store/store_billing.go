package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"
	"time"
)

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
