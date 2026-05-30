# syntax=docker/dockerfile:1.7

FROM node:22-bookworm-slim AS frontend
WORKDIR /src
ARG LIKEABLE_BUILD_GIT_COMMIT_SHA=unknown
RUN npm install -g bun@1.3.10
COPY package.json bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile
COPY tsconfig.json vite.config.ts postcss.config.js tailwind.config.js ./
RUN printf '%s\n' "$LIKEABLE_BUILD_GIT_COMMIT_SHA" > /tmp/likeable-build-revision
COPY frontend ./frontend
RUN node node_modules/vite/bin/vite.js build

FROM golang:1.26.1-bookworm AS backend
WORKDIR /src
ARG LIKEABLE_BUILD_GIT_COMMIT_SHA=unknown
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
RUN printf '%s\n' "$LIKEABLE_BUILD_GIT_COMMIT_SHA" > /tmp/likeable-build-revision
COPY cmd ./cmd
COPY internal ./internal
COPY --from=frontend /src/dist ./internal/likeable/web-dist
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid=" -o /out/likeable ./cmd/likeable

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates curl git \
  && rm -rf /var/lib/apt/lists/*
RUN useradd --uid 10001 --create-home --home-dir /home/likeable --shell /usr/sbin/nologin likeable && mkdir -p /data && chown -R likeable:likeable /data
COPY --from=backend /out/likeable /usr/local/bin/likeable
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
USER likeable
ENV ADDR=:8080 DATABASE_PATH=/data/likeable.db HOME=/tmp
EXPOSE 8080
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD curl -fsS http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["likeable"]
