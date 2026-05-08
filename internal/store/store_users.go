package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
)

func (s *Store) UpsertUser(ctx context.Context, email, name, avatar string) (*User, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, errors.New("email is required")
	}
	now := nowString()
	id := uuid.NewString()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users(id, email, name, avatar_url, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(email) DO UPDATE SET
			name=excluded.name,
			avatar_url=excluded.avatar_url,
			updated_at=excluded.updated_at
	`, id, email, strings.TrimSpace(name), strings.TrimSpace(avatar), now, now)
	if err != nil {
		return nil, err
	}
	return s.UserByEmail(ctx, email)
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, email, name, avatar_url, access_status, access_note, created_at FROM users WHERE email = ?`, normalizeEmail(email))
	return scanUser(row)
}

func (s *Store) UserBySessionToken(ctx context.Context, token string) (*User, error) {
	hash := sessionHash(token)
	row := s.db.QueryRowContext(ctx, `
		SELECT users.id, users.email, users.name, users.avatar_url, users.access_status, users.access_note, users.created_at
		FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = ? AND sessions.expires_at > ?
	`, hash, nowString())
	return scanUser(row)
}

func scanUser(scanner interface{ Scan(...any) error }) (*User, error) {
	var user User
	if err := scanner.Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.AccessStatus, &user.AccessNote, &user.CreatedAt); err != nil {
		return nil, err
	}
	if user.AccessStatus == "" {
		user.AccessStatus = "active"
	}
	return &user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID, token string, ttl time.Duration) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions(token_hash, user_id, expires_at, created_at)
		VALUES(?, ?, ?, ?)
	`, sessionHash(token), userID, time.Now().UTC().Add(ttl).Format(time.RFC3339Nano), nowString())
	return err
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, sessionHash(token))
	return err
}

func sessionHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Store) ConfigMap(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM app_config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}

func (s *Store) UpsertConfig(ctx context.Context, values map[string]string, secretKeys map[string]bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			if !secretKeys[key] {
				if _, err := tx.ExecContext(ctx, `DELETE FROM app_config WHERE key = ?`, key); err != nil {
					return err
				}
			}
			continue
		}
		secret := 0
		if secretKeys[key] {
			secret = 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO app_config(key, value, secret, updated_at)
			VALUES(?, ?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value, secret=excluded.secret, updated_at=excluded.updated_at
		`, key, value, secret, nowString()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpdateUserAccess(ctx context.Context, userID, status, note string) (*User, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "", "active":
		status = "active"
	case "restricted":
	default:
		return nil, fmt.Errorf("invalid access status %q", status)
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET access_status = ?, access_note = ?, updated_at = ?
		WHERE id = ?
	`, status, strings.TrimSpace(note), nowString(), userID)
	if err != nil {
		return nil, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, sql.ErrNoRows
	}
	return s.UserByID(ctx, userID)
}

func (s *Store) UserByID(ctx context.Context, userID string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, email, name, avatar_url, access_status, access_note, created_at FROM users WHERE id = ?`, userID)
	return scanUser(row)
}

func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) RemoveEmailFromSignupAllowlist(ctx context.Context, email string) error {
	cfg, err := s.ConfigMap(ctx)
	if err != nil {
		return err
	}
	current := strings.TrimSpace(cfg["signup_allowed_emails"])
	if current == "" {
		return nil
	}
	next := removeExactEmailFromListConfig(current, email)
	if next == "" {
		_, err = s.db.ExecContext(ctx, `DELETE FROM app_config WHERE key = 'signup_allowed_emails'`)
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE app_config SET value = ?, updated_at = ? WHERE key = 'signup_allowed_emails'`, next, nowString())
	return err
}
