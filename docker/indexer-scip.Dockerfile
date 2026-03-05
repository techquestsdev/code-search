# Build zoekt-git-index
FROM golang:1.24-alpine AS zoekt-builder

WORKDIR /zoekt

# Install build dependencies
RUN apk add --no-cache git
RUN git clone --depth 1 https://github.com/sourcegraph/zoekt.git .
RUN go build -o /bin/zoekt-git-index ./cmd/zoekt-git-index

# Build indexer
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source
COPY . .

# Build indexer
RUN CGO_ENABLED=0 go build -o /bin/indexer ./cmd/indexer

# Build ctags in a separate stage (cleaner)
FROM alpine:3.19 AS ctags-builder

WORKDIR /tmp

# Copy the install script
COPY --from=zoekt-builder /zoekt/install-ctags-alpine.sh /tmp/install-ctags-alpine.sh
RUN chmod +x /tmp/install-ctags-alpine.sh

# Run the installation
RUN /tmp/install-ctags-alpine.sh

# Build scip-go binary
FROM golang:1.24-alpine AS scip-go-builder

RUN apk add --no-cache git
RUN go install github.com/sourcegraph/scip-go@latest

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
# - git, ca-certificates, tzdata, jansson: base indexer requirements
# - nodejs, npm: for scip-typescript (via npx)
# - python3, py3-pip: for scip-python
RUN apk add --no-cache ca-certificates tzdata git jansson \
    nodejs npm \
    python3 py3-pip

# Install scip-python
RUN pip install --break-system-packages scip-python

WORKDIR /app

COPY --from=builder /bin/indexer /app/indexer
COPY --from=zoekt-builder /bin/zoekt-git-index /app/zoekt-git-index
COPY --from=ctags-builder /usr/local/bin/universal-ctags /app/universal-ctags
COPY --from=scip-go-builder /go/bin/scip-go /app/scip-go

# Create directories
RUN mkdir -p /data/repos /data/index /data/scip

# Create non-root user
RUN adduser -D -g '' code-search
RUN chown -R 1000:1000 /data /app
USER 1000:1000

ENV PATH="/app:${PATH}"

ENTRYPOINT ["/app/indexer"]
