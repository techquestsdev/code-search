# Build stage
FROM oven/bun:latest AS builder

# Build argument for API URL (baked in at build time)
ARG NEXT_PUBLIC_API_URL=https://code-search.techquestslabs.dev
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}

WORKDIR /app

# Copy package files
COPY web/package.json web/bun.lock  ./
RUN bun install --frozen-lockfile

# Copy source
COPY web/ ./

# Build
RUN bun run build

# Runtime stage
FROM oven/bun:alpine

WORKDIR /app

# Copy built assets
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public

# Use non-root user
RUN adduser -D -g '' code-search
RUN chown -R 1000:1000 /app
USER 1000:1000

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["bun", "server.js"]
