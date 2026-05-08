# syntax=docker/dockerfile:1.7

FROM node:22-bookworm-slim AS frontend
WORKDIR /src
RUN npm install -g bun@1.3.10
COPY package.json bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile
COPY tsconfig.json vite.config.ts postcss.config.js tailwind.config.js ./
COPY frontend ./frontend
RUN node node_modules/vite/bin/vite.js build

FROM golang:1.24-bookworm AS backend
WORKDIR /src
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend /src/dist ./internal/likeable/web-dist
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o /out/likeable ./cmd/likeable

FROM debian:bookworm-slim AS fibe-cli
RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates curl jq tar gzip \
  && rm -rf /var/lib/apt/lists/*
RUN set -eu; \
  arch="$(dpkg --print-architecture)"; \
  case "$arch" in \
    amd64) fibe_arch="amd64" ;; \
    arm64) fibe_arch="arm64" ;; \
    *) echo "unsupported architecture: $arch" >&2; exit 1 ;; \
  esac; \
  release="$(curl -fsSL https://api.github.com/repos/fibegg/sdk/releases/latest)"; \
  version="$(printf '%s' "$release" | jq -r '.tag_name | sub("^v"; "")')"; \
  asset="fibe_${version}_linux_${fibe_arch}.tar.gz"; \
  url="$(printf '%s' "$release" | jq -r --arg asset "$asset" '.assets[] | select(.name == $asset) | .browser_download_url')"; \
  test -n "$url"; \
  curl -fsSL "$url" -o /tmp/fibe.tar.gz; \
  tar -xzf /tmp/fibe.tar.gz -C /tmp; \
  install -m 0755 /tmp/fibe /usr/local/bin/fibe; \
  rm -rf /tmp/fibe /tmp/fibe.tar.gz

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates curl git \
  && rm -rf /var/lib/apt/lists/*
RUN useradd --uid 10001 --create-home --home-dir /home/likeable --shell /usr/sbin/nologin likeable && mkdir -p /data && chown -R likeable:likeable /data
COPY --from=backend /out/likeable /usr/local/bin/likeable
COPY --from=fibe-cli /usr/local/bin/fibe /usr/local/bin/fibe
USER likeable
ENV ADDR=:8080 DATABASE_PATH=/data/likeable.db HOME=/tmp
EXPOSE 8080
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD curl -fsS http://127.0.0.1:8080/healthz || exit 1
CMD ["likeable"]
