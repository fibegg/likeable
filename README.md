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

Likeable talks to Fibe through the Go SDK built into the application binary; the runtime image does not install or shell out to the `fibe` CLI.

After the first admin login, open Admin and configure:

- Fibe base URL, API key, template version, and agent/server pool.
- Google OAuth for sign in.
- GitHub OAuth for repository export.
- Stripe prices and webhook secret.
- SMTP delivery.
- Signup mode, free build minutes/window, project cap, paid playground slot duration, and production project duration.

Signup defaults to forbidden until the admin changes it.

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

To run the fully containerized app instead of the live-reload development server:

```bash
docker compose --profile app up --build
```

Before deploying a test build, run the deploy preflight:

```bash
bin/deploy-preflight
```

It runs the TypeScript check, frontend build, Go tests, production Docker image build, and a container smoke against `/healthz` and the admin billing health API. Override the image tag or smoke port with `LIKEABLE_IMAGE_TAG` and `LIKEABLE_PREFLIGHT_PORT`.

## Test Deployment Runbook

Use this flow for the Vyakymenko Likeable test environment or any equivalent test VPS.

The automated path is:

```bash
LIKEABLE_DEPLOY_HOST=user@server.example.com \
LIKEABLE_IMAGE_TAG=registry.example.com/likeable:test \
LIKEABLE_BASE_URL=https://likeable.example.com \
LIKEABLE_ADMIN_EMAIL=admin@example.com \
bin/deploy-vps
```

`bin/deploy-vps` runs the local preflight, pushes the image, updates Redis and the app container over SSH, then waits for `/healthz`. Set `LIKEABLE_RUN_PREFLIGHT=0` or `LIKEABLE_PUSH_IMAGE=0` only when the image was already tested or already pushed.

The manual path is:

1. Run the local preflight and tag the exact image you want to deploy:

```bash
LIKEABLE_IMAGE_TAG=registry.example.com/likeable:test bin/deploy-preflight
docker push registry.example.com/likeable:test
```

2. Run Redis and the app container with a persistent database volume:

```bash
docker network create likeable-net || true
docker run -d --name likeable-redis \
  --restart unless-stopped \
  --network likeable-net \
  -v likeable_redis:/data \
  redis:7.4-alpine redis-server --appendonly yes --save 60 1

docker run -d --name likeable \
  --restart unless-stopped \
  --network likeable-net \
  -p 8080:8080 \
  -v likeable_data:/data \
  -e ADDR=:8080 \
  -e DATABASE_PATH=/data/likeable.db \
  -e BASE_URL=https://likeable.example.com \
  -e ADMIN_EMAIL=admin@example.com \
  -e REDIS_URL=redis://likeable-redis:6379/0 \
  registry.example.com/likeable:test
```

Put TLS and the public hostname in front of port `8080` with your reverse proxy. For a single small test host, the default `LIKEABLE_ROLE=all` is fine. On a busier host, run one `LIKEABLE_ROLE=web` container behind HTTP and one `LIKEABLE_ROLE=worker` container against the same database and Redis.

3. Bootstrap admin access.

For a private smoke only, temporarily set `LIKEABLE_DEV_AUTH=1` and sign in with `ADMIN_EMAIL`. For a public test URL, set `LIKEABLE_BOOTSTRAP_TOKEN` once, POST Google OAuth config to `/api/bootstrap/config`, then remove the token and restart before inviting users.

4. Configure Admin.

Set Fibe, Google OAuth, GitHub export, SMTP, signup mode, free minutes/window, project cap, paid project-slot duration, production-project duration, and Stripe:

- `stripe_secret_key`
- `stripe_webhook_secret`
- `stripe_price_id_1_hour`, `stripe_price_id_10_hours`, `stripe_price_id_100_hours`
- `stripe_project_quota_price_id`
- `stripe_production_project_price_id`

Stripe webhook URL: `${BASE_URL}/api/stripe/webhook`.

Checkout uses backend-created Stripe Checkout Sessions and does not require `stripe_publishable_key`. It also uses Stripe dynamic payment methods by not sending `payment_method_types`. Enable Link in the Stripe Dashboard payment method settings; the app will not override that Dashboard configuration.

5. Smoke the user flows.

- Admin billing health has no blocking issues.
- A new user can sign in, create a project, and send a first message.
- Sending a message to a stopped project wakes the playground and then sends the prompt.
- Hour-pack checkout grants build minutes.
- Project-slot checkout increases the project cap for the configured number of days.
- Production-project checkout pins that project as always-on until expiry, blocks manual stop, lets the user save a custom domain request, and shows the CNAME target in the project menu.
- Admin diagnostics for a user project shows conversation, agent, playground, server, repositories, payments, hour ledger, and work sessions.

Domain DNS is currently manual: after a production-project purchase, the user can save the intended custom domain and the app shows the target host to use as the customer CNAME. Admin diagnostics include the saved domain, status, and target. DNS ownership verification and automatic domain provisioning are separate follow-up work.

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
