package store

import (
	"context"
	"database/sql"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
	db      *sql.DB
	dataDir string
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db, dataDir: filepath.Dir(path)}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DataDir() string { return s.dataDir }

func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA foreign_keys=ON`,
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			avatar_url TEXT NOT NULL DEFAULT '',
			access_status TEXT NOT NULL DEFAULT 'active',
			access_note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS app_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			secret INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			conversation_id TEXT NOT NULL UNIQUE,
			agent_id TEXT NOT NULL DEFAULT '',
			marquee_id TEXT NOT NULL DEFAULT '',
			playground_id TEXT NOT NULL DEFAULT '',
			playground_name TEXT NOT NULL DEFAULT '',
			playspec_id TEXT NOT NULL DEFAULT '',
			prop_id TEXT NOT NULL DEFAULT '',
			repo_url TEXT NOT NULL DEFAULT '',
			preview_url TEXT NOT NULL DEFAULT '',
			selected_service_name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'creating',
			error_message TEXT NOT NULL DEFAULT '',
			provisioning_lock_until TEXT NOT NULL DEFAULT '',
			cleanup_last_error TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_projects_user_updated ON projects(user_id, updated_at DESC)`,
		`CREATE TABLE IF NOT EXISTS project_repositories (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			role TEXT NOT NULL DEFAULT '',
			prop_id TEXT NOT NULL DEFAULT '',
			repo_url TEXT NOT NULL DEFAULT '',
			source_repo_url TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			service_names TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_repositories_project ON project_repositories(project_id)`,
		`CREATE TABLE IF NOT EXISTS project_services (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			url TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT '',
			visibility TEXT NOT NULL DEFAULT '',
			auth_required INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			UNIQUE(project_id, name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_services_project ON project_services(project_id)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			body TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_project_created ON messages(project_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS message_attachments (
			id TEXT PRIMARY KEY,
			message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			filename TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT '',
			size INTEGER NOT NULL DEFAULT 0,
			storage_path TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_message_attachments_message ON message_attachments(message_id)`,
		`CREATE INDEX IF NOT EXISTS idx_message_attachments_project ON message_attachments(project_id)`,
		`CREATE TABLE IF NOT EXISTS social_connections (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider TEXT NOT NULL,
			provider_user_id TEXT NOT NULL DEFAULT '',
			access_token TEXT NOT NULL,
			scope TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(user_id, provider)
		)`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			stripe_customer_id TEXT NOT NULL DEFAULT '',
			stripe_subscription_id TEXT NOT NULL DEFAULT '',
			current_period_end TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS payments (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider_payment_id TEXT NOT NULL UNIQUE,
			amount_cents INTEGER NOT NULL DEFAULT 0,
			currency TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_payments_user_created ON payments(user_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS user_notices (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			sender TEXT NOT NULL DEFAULT 'admin',
			severity TEXT NOT NULL DEFAULT 'info',
			body TEXT NOT NULL,
			read_at TEXT NOT NULL DEFAULT '',
			dismissed_at TEXT NOT NULL DEFAULT '',
			unsent_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_notices_user_created ON user_notices(user_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS message_credit_ledger (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			delta INTEGER NOT NULL,
			reason TEXT NOT NULL,
			payment_id TEXT NOT NULL DEFAULT '',
			message_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_message_credit_ledger_user_created ON message_credit_ledger(user_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS project_quota_ledger (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			payment_id TEXT NOT NULL DEFAULT '',
			slots INTEGER NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(payment_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_quota_user_expires ON project_quota_ledger(user_id, expires_at DESC)`,
		`CREATE TABLE IF NOT EXISTS export_jobs (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			target_repo_url TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS project_archives (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			project_id TEXT NOT NULL,
			project_title TEXT NOT NULL,
			storage_path TEXT NOT NULL,
			status TEXT NOT NULL,
			github_repo_url TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_archives_user_created ON project_archives(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_project_archives_expires ON project_archives(expires_at)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "projects", "playspec_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "projects", "playground_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "projects", "agent_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "projects", "marquee_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "projects", "prop_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "projects", "error_message", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "projects", "selected_service_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "projects", "provisioning_lock_until", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "projects", "cleanup_last_error", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "users", "access_status", "TEXT NOT NULL DEFAULT 'active'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "users", "access_note", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "user_notices", "sender", "TEXT NOT NULL DEFAULT 'admin'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "user_notices", "read_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "user_notices", "dismissed_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "user_notices", "unsent_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table, name, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName, columnType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if columnName == name {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+name+` `+definition)
	return err
}

func nowString() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func removeExactEmailFromListConfig(raw, email string) string {
	email = normalizeEmail(email)
	var out []string
	seen := map[string]bool{}
	for _, item := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		item = normalizeEmail(item)
		if item == "" || item == email || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return strings.Join(out, "\n")
}
