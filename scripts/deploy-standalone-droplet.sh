#!/usr/bin/env bash
set -Eeuo pipefail

REPO_URL="${REPO_URL:-https://github.com/fibegg/likeable.git}"
BRANCH="${BRANCH:-experiment/codex-droplet-standalone}"
APP_DIR="${APP_DIR:-/opt/likeable}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
OPENAI_MODEL="${OPENAI_MODEL:-gpt-5-mini}"
LIKEABLE_DEV_AUTH="${LIKEABLE_DEV_AUTH:-1}"
LIKEABLE_HTTP_PORT="${LIKEABLE_HTTP_PORT:-8080}"

log() {
  printf '[likeable-deploy] %s\n' "$*"
}

run_as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  else
    sudo "$@"
  fi
}

compose() {
  local compose_files=(-f "$APP_DIR/docker-compose.yml")
  if [ -f "$APP_DIR/docker-compose.traefik.yml" ]; then
    compose_files+=(-f "$APP_DIR/docker-compose.traefik.yml")
  fi
  if [ -f "$APP_DIR/docker-compose.proxy.yml" ]; then
    compose_files+=(-f "$APP_DIR/docker-compose.proxy.yml")
  fi
  run_as_root docker compose "${compose_files[@]}" --project-directory "$APP_DIR" "$@"
}

install_base_packages() {
  if command -v git >/dev/null 2>&1 && command -v curl >/dev/null 2>&1; then
    return
  fi

  log "installing base packages"
  run_as_root apt-get update
  run_as_root env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates curl git
}

install_docker() {
  if command -v docker >/dev/null 2>&1 && run_as_root docker compose version >/dev/null 2>&1; then
    return
  fi

  log "installing Docker"
  curl -fsSL https://get.docker.com | run_as_root sh
}

public_ip() {
  curl -fsS --max-time 2 http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address 2>/dev/null \
    || hostname -I 2>/dev/null | awk '{print $1}'
}

public_url() {
  if [ -n "${LIKEABLE_PUBLIC_URL:-}" ]; then
    printf '%s\n' "$LIKEABLE_PUBLIC_URL"
    return
  fi
  if [ -n "${PUBLIC_URL:-}" ]; then
    printf '%s\n' "$PUBLIC_URL"
    return
  fi

  local ip
  ip="$(public_ip)"
  if [ -z "$ip" ]; then
    log "could not infer public IP; set LIKEABLE_PUBLIC_URL or PUBLIC_URL"
    exit 1
  fi
  printf 'http://%s:8080\n' "$ip"
}

checkout_source() {
  if [ -d "$APP_DIR/.git" ]; then
    log "updating $APP_DIR"
    git -C "$APP_DIR" fetch origin "$BRANCH"
    git -C "$APP_DIR" checkout "$BRANCH" || git -C "$APP_DIR" checkout -b "$BRANCH" "origin/$BRANCH"
    git -C "$APP_DIR" pull --ff-only origin "$BRANCH"
    return
  fi

  log "checking out $REPO_URL#$BRANCH into $APP_DIR"
  run_as_root mkdir -p "$APP_DIR"
  run_as_root chown -R "$(id -u):$(id -g)" "$APP_DIR"
  git -C "$APP_DIR" init
  git -C "$APP_DIR" remote add origin "$REPO_URL"
  git -C "$APP_DIR" fetch origin "$BRANCH"
  git -C "$APP_DIR" checkout -b "$BRANCH" "origin/$BRANCH"
}

write_env() {
  local env_file="$APP_DIR/.env"
  local url
  url="$(public_url)"

  local existing_openai_key=""
  if [ -f "$env_file" ]; then
    existing_openai_key="$(awk -F= '/^OPENAI_API_KEY=/{print substr($0, length($1) + 2)}' "$env_file" | tail -n 1)"
  fi

  local openai_key="${OPENAI_API_KEY:-$existing_openai_key}"
  if [ -z "$openai_key" ]; then
    log "OPENAI_API_KEY is empty; app will start, but generation will fail until the key is saved in .env or Admin"
  else
    log "OPENAI_API_KEY is present"
  fi

  log "writing $env_file for $url"
  umask 077
  {
    printf 'LIKEABLE_PUBLIC_URL=%s\n' "$url"
    printf 'LIKEABLE_HTTP_PORT=%s\n' "$LIKEABLE_HTTP_PORT"
    printf 'ADMIN_EMAIL=%s\n' "$ADMIN_EMAIL"
    printf 'OPENAI_MODEL=%s\n' "$OPENAI_MODEL"
    printf 'OPENAI_API_KEY=%s\n' "$openai_key"
    printf 'LIKEABLE_WORKSPACE_ROOT=/data/workspaces\n'
    printf 'LIKEABLE_DEV_AUTH=%s\n' "$LIKEABLE_DEV_AUTH"
  } > "$env_file"
}

write_traefik_override() {
  local override_file="$APP_DIR/docker-compose.traefik.yml"
  local host="${LIKEABLE_TRAEFIK_HOST:-}"
  if [ -z "$host" ]; then
    rm -f "$override_file"
    return
  fi

  local network="${LIKEABLE_TRAEFIK_NETWORK:-marquee-traefik-32_default}"
  local certresolver="${LIKEABLE_TRAEFIK_CERTRESOLVER:-letsencrypt}"
  log "writing Traefik route for $host on network $network"
  cat > "$override_file" <<YAML
services:
  likeable:
    labels:
      "traefik.enable": "true"
      "traefik.docker.network": "$network"
      "traefik.http.middlewares.likeable-standalone-redirect-to-https.redirectscheme.scheme": "https"
      "traefik.http.routers.likeable-standalone-http.entrypoints": "web"
      "traefik.http.routers.likeable-standalone-http.middlewares": "likeable-standalone-redirect-to-https"
      "traefik.http.routers.likeable-standalone-http.rule": "Host(\`$host\`)"
      "traefik.http.routers.likeable-standalone-http.service": "likeable-standalone"
      "traefik.http.routers.likeable-standalone-secure.entrypoints": "websecure"
      "traefik.http.routers.likeable-standalone-secure.rule": "Host(\`$host\`)"
      "traefik.http.routers.likeable-standalone-secure.service": "likeable-standalone"
      "traefik.http.routers.likeable-standalone-secure.tls": "true"
      "traefik.http.routers.likeable-standalone-secure.tls.certresolver": "$certresolver"
      "traefik.http.services.likeable-standalone.loadbalancer.healthcheck.path": "/healthz"
      "traefik.http.services.likeable-standalone.loadbalancer.healthcheck.interval": "30s"
      "traefik.http.services.likeable-standalone.loadbalancer.healthcheck.timeout": "5s"
      "traefik.http.services.likeable-standalone.loadbalancer.server.port": "8080"
    networks:
      - default
      - traefik_proxy

networks:
  traefik_proxy:
    external: true
    name: "$network"
YAML
}

write_caddy_proxy_override() {
  local hosts="${LIKEABLE_CADDY_HOSTS:-}"
  local override_file="$APP_DIR/docker-compose.proxy.yml"
  local caddyfile="$APP_DIR/Caddyfile"
  if [ -z "$hosts" ]; then
    rm -f "$override_file" "$caddyfile"
    return
  fi

  local site_hosts
  site_hosts="$(printf '%s' "$hosts" | tr ',' ' ' | awk '{$1=$1; print}' | sed 's/ /, /g')"
  if [ -z "$site_hosts" ]; then
    rm -f "$override_file" "$caddyfile"
    return
  fi

  log "writing Caddy proxy route for $site_hosts"
  cat > "$caddyfile" <<CADDY
{
	email ${LIKEABLE_CADDY_EMAIL:-admin@example.com}
}

$site_hosts {
	encode zstd gzip
	reverse_proxy likeable:8080
}
CADDY

  cat > "$override_file" <<YAML
services:
  caddy:
    image: caddy:2.9-alpine
    restart: unless-stopped
    depends_on:
      likeable:
        condition: service_started
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config

volumes:
  caddy_data:
  caddy_config:
YAML
}

reset_data_if_requested() {
  if [ "${LIKEABLE_RESET_DATA:-0}" != "1" ]; then
    return
  fi
  log "resetting standalone containers and volumes"
  compose --profile app down -v --remove-orphans || true
}

open_firewall() {
  if command -v ufw >/dev/null 2>&1 && run_as_root ufw status | grep -q '^Status: active'; then
    log "allowing tcp/$LIKEABLE_HTTP_PORT in ufw for direct test access"
    run_as_root ufw allow "$LIKEABLE_HTTP_PORT/tcp" >/dev/null
  fi
}

compose_up() {
  log "building and starting Docker Compose stack"
  compose --profile app up --build -d
}

wait_for_health() {
  log "waiting for health check"
  for _ in $(seq 1 60); do
    if curl -fsS "http://127.0.0.1:${LIKEABLE_HTTP_PORT}/healthz" >/dev/null; then
      log "health check passed"
      return
    fi
    sleep 2
  done

  log "health check failed; recent logs:"
  compose --profile app logs --tail=120 likeable
  exit 1
}

main() {
  install_base_packages
  install_docker
  checkout_source
  write_env
  write_traefik_override
  write_caddy_proxy_override
  open_firewall
  reset_data_if_requested
  compose_up
  wait_for_health

  log "running containers:"
  compose --profile app ps
  log "public URL: $(public_url)"
}

main "$@"
