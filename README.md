# Likeable

Standalone app builder with a Go backend, React frontend, local project workspaces, and OpenAI-powered code generation.

## Standalone Runtime

This branch runs as a standalone droplet app:

- Project source files live on the droplet under `LIKEABLE_WORKSPACE_ROOT` or `DATABASE_PATH`'s sibling `workspaces/` directory.
- Preview URLs are served by Likeable itself at `/api/projects/:id/preview/`.
- The build agent calls the OpenAI Responses API with `OPENAI_API_KEY`.
- ZIP export and GitHub export read directly from the local workspace.
- Redis still backs rate limits, background jobs, cleanup jobs, and email delivery.

Required runtime settings:

- `BASE_URL`: public app URL for direct process runs, for example `https://likeable.example.com`.
- `LIKEABLE_PUBLIC_URL`: public app URL used by `docker-compose.yml`; it is mapped into the container as `BASE_URL`.
- `DATABASE_PATH`: SQLite database path, default `/data/likeable.db` in Docker.
- `REDIS_URL`: Redis URL.
- `OPENAI_API_KEY`: optional env bootstrap; can also be saved in Admin as `openai_api_key`.
- `OPENAI_MODEL`: optional, default `gpt-5-mini`. Use `gpt-5.2-codex` from Admin for stronger coding passes if the key has access.
- `LIKEABLE_WORKSPACE_ROOT`: optional, default `/data/workspaces` in Docker.

After the first admin login, open Admin and configure:

- OpenAI API key, model, and workspace root.
- Google OAuth for sign in.
- GitHub OAuth or shared token for repository export.
- Stripe prices and webhook secret.
- SMTP delivery.
- Signup mode, free build hours, and project cap.

Signup defaults to forbidden until the admin changes it.

## Droplet Plan

1. Point the domain at the droplet and terminate TLS with a reverse proxy such as Caddy or Nginx.
2. Deploy the Docker Compose stack with persistent volumes for `/data` and Redis.
3. Set `LIKEABLE_PUBLIC_URL=https://your-domain`, `OPENAI_API_KEY`, and optionally `OPENAI_MODEL`.
4. Run `docker compose --profile app up --build -d`.
5. Sign in as `ADMIN_EMAIL`, open Admin, save the remaining provider settings, then switch signup from `forbidden` when ready.
6. Create a test playground. The project should create `/data/workspaces/<project-id>/index.html`, serve preview from your domain, and export ZIP/GitHub from that local folder.

For a fresh Ubuntu/Debian droplet, the deploy script handles the same flow:

```bash
OPENAI_API_KEY=sk-... LIKEABLE_PUBLIC_URL=http://your-droplet-ip:8080 scripts/deploy-standalone-droplet.sh
```

If `8080` is already taken on a shared test droplet, set `LIKEABLE_HTTP_PORT=18080` and include that port in `LIKEABLE_PUBLIC_URL`.

## Local Development

```bash
cp .env.example .env
docker compose up
bin/dev
```

Open `http://localhost:5173`. `docker compose up` starts Redis. `bin/dev` starts the Go web server on `localhost:8080`, the Asynq worker, and the Vite frontend with hot reload on `localhost:5173`.

For local development without Google OAuth, `bin/dev` enables `ADMIN_EMAIL=admin@example.com` and `LIKEABLE_DEV_AUTH=1` by default. The login page shows a `Dev sign in` button, or you can POST `/api/dev/login?email=admin@example.com`.

To run the fully containerized app instead of the live-reload development server:

```bash
docker compose --profile app up --build
```

## GitHub Integration

Likeable does not need any GitHub webhooks.

GitHub is used only as an OAuth + REST/git export integration:

- Users connect GitHub through `/api/profile/github/start`.
- GitHub redirects back to `/api/profile/github/callback`.
- Likeable stores the OAuth access token and later uses it to create a repository and push exported project code.

Configure a GitHub OAuth App, not a GitHub App webhook:

- Homepage URL: the public Likeable `BASE_URL`.
- Authorization callback URL: `${BASE_URL}/api/profile/github/callback`.
- Requested OAuth scopes: `repo` and `workflow`. The `workflow` scope is required because exported projects can include `.github/workflows/*`.
- Client ID: save in Admin as `github_client_id`.
- Client secret: save in Admin as `github_client_secret`.

For installations that should export without per-user OAuth, Admin can also store a shared export credential:

- Username: save in Admin as `github_username`.
- Personal access token: save in Admin as `github_token`. The token must be able to create repositories and push workflow files.

The same fallback can also come from `GITHUB_USERNAME`/`GITHUB_TOKEN` or `GH_USERNAME`/`GH_TOKEN` environment variables when Admin config is empty.

Do not configure repository webhooks for Likeable. There is no handler for GitHub `push`, `pull_request`, `workflow_run`, or installation events. The only inbound provider webhook currently handled by Likeable is Stripe at `/api/stripe/webhook`.

## Backend Shape

The Go backend is a small modular monolith. Directories are package boundaries, not cosmetic folders:

- `internal/domain`: shared application data types and tiny pure helpers.
- `internal/store`: SQLite schema, migrations, and persistence methods. It depends on `domain` only.
- `internal/workspace`: local workspace creation, preview probing, OpenAI build-agent calls, messages, activity, and resource cleanup.
- `internal/project`: pure project naming and agent prompt/context helpers.
- `internal/likeable`: HTTP routes, handlers, runtime wiring, jobs, quota policy, billing, auth, and orchestration.

Keep dependencies pointing inward: handlers orchestrate `store`, `workspace`, and pure project/domain helpers; lower-level packages should not import `internal/likeable`.
