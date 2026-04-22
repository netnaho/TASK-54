# ============================================================
# Stage 1 — Builder
# ============================================================
FROM golang:1.22-alpine AS builder

# wget for vendoring HTMX at build time
RUN apk add --no-cache wget ca-certificates

WORKDIR /src

# Download Go module dependencies before copying source so that
# the dependency layer is cached independently of source changes.
COPY go.mod go.sum ./
RUN go mod download

# Copy source tree (see .dockerignore for exclusions)
COPY . .

# Fetch HTMX and write it into the source tree before building.
# This is a build-time internet fetch; the runtime image has no internet access.
RUN mkdir -p web/static/vendor && \
    wget -q -O web/static/vendor/htmx.min.js \
      https://unpkg.com/htmx.org@1.9.12/dist/htmx.min.js

# Build the statically linked binary.
# CGO_ENABLED=0 is safe here because modernc.org/sqlite is pure Go.
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/server \
      ./cmd/server

# ============================================================
# Stage 2 — Runtime
# ============================================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /out/server .

# Copy runtime-needed assets; the binary does NOT embed these.
COPY --from=builder /src/migrations  ./migrations
COPY --from=builder /src/seeds       ./seeds
COPY --from=builder /src/web         ./web

# Create data directory structure; real persistence comes from Docker volumes.
RUN mkdir -p \
      /data/db \
      /data/media \
      /data/exports \
      /data/diagnostics \
      /data/config_snapshots

# Configuration — all overridable via docker-compose environment section
ENV PORT=8080 \
    DB_PATH=/data/db/careops.db \
    MIGRATIONS_PATH=/app/migrations \
    SEEDS_PATH=/app/seeds \
    TEMPLATES_PATH=/app/web/templates \
    STATIC_PATH=/app/web/static \
    MEDIA_PATH=/data/media \
    EXPORTS_PATH=/data/exports \
    DIAGNOSTICS_PATH=/data/diagnostics \
    CONFIG_SNAP_PATH=/data/config_snapshots \
    SESSION_TTL_HOURS=24 \
    COOKIE_NAME=careops_session \
    LOG_LEVEL=info \
    LOG_FORMAT=json \
    APP_ENV=production

EXPOSE 8080

# Non-root user for runtime security
RUN addgroup -S careops && adduser -S -G careops careops
RUN chown -R careops:careops /data /app
USER careops

ENTRYPOINT ["/app/server"]
