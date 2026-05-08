# Likeable

Fibe-playground-ready app builder with a Go backend and React frontend.

## Local

```bash
cp .env.example .env
docker compose up
bin/dev
```

Open `http://localhost:5173`. `docker compose up` starts Redis. `bin/dev` starts the Go web server on `localhost:8080`, the Asynq worker, and the Vite frontend with hot reload on `localhost:5173`.

For local development without Google OAuth, `bin/dev` enables `ADMIN_EMAIL=admin@example.com` and `LIKEABLE_DEV_AUTH=1` by default. The login page shows a `Dev sign in` button, or you can POST `/api/dev/login?email=admin@example.com`.

The compose stack includes Redis. Redis backs request rate limits and Asynq background jobs for project provisioning, deletion, quota sweeps, archive cleanup, and email delivery.

Runtime environment is intentionally small: `BASE_URL`, `ADMIN_EMAIL`, `REDIS_URL`, and local-only `LIKEABLE_DEV_AUTH`. Product settings are configured from Admin, not environment variables.

After the first admin login, open Admin and configure:

- Fibe base URL, API key, template version, and agent/server pool.
- Google OAuth for sign in.
- GitHub OAuth for repository export.
- Stripe prices and webhook secret.
- SMTP delivery.
- Signup mode, free daily messages, and project cap.

Signup defaults to forbidden until the admin changes it.

To run the fully containerized app instead of the live-reload development server:

```bash
docker compose --profile app up --build
```

## Development With Live Reload

The short development workflow mirrors the main Fibe app:

```bash
docker compose up
bin/dev
```

`bin/dev` installs `air` and `foreman` if needed, then runs `Procfile.dev`.

`Procfile.dev` starts three processes:

- `web`: Go HTTP server with backend live reload.
- `worker`: Asynq background job worker with backend live reload.
- `vite`: React/Vite dev server with hot module reload.

Vite proxies `/api` plus `/healthz` to the Go web server.

## Backend Shape

The Go backend is a small modular monolith. Directories are package boundaries, not cosmetic folders:

- `internal/domain`: shared application data types and tiny pure helpers.
- `internal/store`: SQLite schema, migrations, and persistence methods. It depends on `domain` only.
- `internal/fibe`: the Fibe CLI-backed gateway for playgrounds, conversations, previews, and resource cleanup.
- `internal/project`: pure project naming and agent prompt/context helpers.
- `internal/likeable`: HTTP routes, handlers, runtime wiring, jobs, quota policy, billing, auth, and orchestration.

Keep dependencies pointing inward: handlers orchestrate `store`, `fibe`, and pure project/domain helpers; lower-level packages should not import `internal/likeable`.
