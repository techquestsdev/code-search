# Lightweight sidecar for triggering zoekt index reloads on network filesystems
# (CephFS, NFS, EFS) that don't propagate inotify events reliably.
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code (cmd and internal packages needed)
COPY cmd/zoekt-refresh/ ./cmd/zoekt-refresh/
COPY internal/log/ ./internal/log/

# Build
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/zoekt-refresh ./cmd/zoekt-refresh

# Runtime stage - minimal image
FROM alpine:3.23

# Install ca-certificates for any HTTPS needs
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/zoekt-refresh /app/zoekt-refresh

# Create non-root user matching zoekt container
RUN adduser -D -g '' -u 1000 code-search
USER 1000:1000

# Default environment
ENV INDEX_PATH=/data/index
ENV REFRESH_INTERVAL=30s

ENTRYPOINT ["/app/zoekt-refresh"]
