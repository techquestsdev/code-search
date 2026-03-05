# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

RUN apk add --no-cache gcc musl-dev

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=1 go build -o /bin/api-server ./cmd/api
RUN CGO_ENABLED=0 go build -o /bin/migrate ./cmd/migrate

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/api-server /app/api-server
COPY --from=builder /bin/migrate /app/migrate
COPY --from=builder /app/migrations /app/migrations

# Create non-root user
RUN adduser -D -g '' code-search
RUN chown -R 1000:1000 /app
USER 1000:1000

EXPOSE 8080

ENTRYPOINT ["/app/api-server"]
