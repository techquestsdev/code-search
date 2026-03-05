# Build stage
FROM oven/bun:latest AS builder

WORKDIR /app

# Install dependencies
COPY website/package.json website/bun.lock  ./
RUN bun install --frozen-lockfile

# Copy source
COPY website/ ./

# Build static site
RUN bun run build

# Runtime stage - serve static files with Caddy
FROM caddy:alpine

# Copy built static files
COPY --from=builder /app/dist /dist

# Copy Caddyfile
COPY website/Caddyfile /etc/caddy/Caddyfile

# Remove file capabilities from Caddy so it can run with
# allowPrivilegeEscalation=false (we bind to high port 4321).
# Also create directories with proper permissions for non-root runtime.
RUN apk add --no-cache libcap attr \
  && setcap -r /usr/bin/caddy \
  && setfattr -x security.capability /usr/bin/caddy || true \
  && apk del libcap attr \
  && mkdir -p /config/caddy /data/caddy \
  && chown -R 1000:1000 /config /data

USER 1000:1000

EXPOSE 4321

CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile"]
