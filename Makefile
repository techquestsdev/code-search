.PHONY: all build clean test lint help

# Configuration
GOCMD := go
DOCKER_COMPOSE := docker compose
DOCKER := docker
BIN_DIR := bin
CMD_DIR := cmd

# Environment for local development
DEV_ENV := CS_DATABASE_URL=postgres://codesearch:codesearch@localhost:5432/codesearch?sslmode=disable \
	CS_REDIS_ADDR=localhost:6379 \
	CS_ZOEKT_URL=http://localhost:6070 \
	CS_REPOS_BASE_PATH=./data/repos \
	CS_INDEXER_INDEX_PATH=./data/index \
	CS_INDEXER_REPOS_PATH=./data/repos \
	CS_LOG_LEVEL=debug

# =============================================================================
# Build
# =============================================================================

all: build

build: ## Build all binaries
	$(GOCMD) build -o $(BIN_DIR)/code-search ./$(CMD_DIR)/cli
	$(GOCMD) build -o $(BIN_DIR)/api-server ./$(CMD_DIR)/api
	$(GOCMD) build -o $(BIN_DIR)/indexer ./$(CMD_DIR)/indexer
	$(GOCMD) build -o $(BIN_DIR)/migrate ./$(CMD_DIR)/migrate
	$(GOCMD) build -o $(BIN_DIR)/zoekt-refresh ./$(CMD_DIR)/zoekt-refresh
	$(GOCMD) build -o $(BIN_DIR)/mcp-server ./$(CMD_DIR)/mcp

# =============================================================================
# Development
# =============================================================================

dev-api: ## Run API server locally
	$(DEV_ENV) $(GOCMD) run ./$(CMD_DIR)/api

dev-indexer: ## Run indexer locally
	$(DEV_ENV) $(GOCMD) run ./$(CMD_DIR)/indexer

dev-web: ## Run web frontend locally
	cd web && bun dev

dev-website: ## Run project website locally
	cd website && npm run dev

dev-zoekt: ## Run zoekt-webserver locally
	@mkdir -p ./data/index
	zoekt-webserver -index ./data/index -listen :6070

dev-mcp: ## Run MCP server (stdio)
	$(GOCMD) run ./$(CMD_DIR)/mcp --api-url http://localhost:8080

dev-mcp-http: ## Run MCP server (HTTP on :9090)
	$(GOCMD) run ./$(CMD_DIR)/mcp --api-url http://localhost:8080 --transport http --http-addr :9090

dev-infra: ## Start infrastructure (postgres, redis, zoekt)
	$(DOCKER_COMPOSE) up -d postgres redis zoekt

dev-setup: ## First-time setup for local development
	@echo "==> Installing zoekt binaries..."
	go install github.com/sourcegraph/zoekt/cmd/zoekt-webserver@latest
	go install github.com/sourcegraph/zoekt/cmd/zoekt-git-index@latest
	@echo "==> Installing web dependencies..."
	cd web && bun install
	@echo "==> Downloading Go dependencies..."
	$(GOCMD) mod download
	@echo "==> Starting infrastructure..."
	$(DOCKER_COMPOSE) up -d postgres redis
	@sleep 3
	@echo "==> Running migrations..."
	-$(GOCMD) run ./$(CMD_DIR)/migrate up
	@echo ""
	@echo "✅ Setup complete! Run in separate terminals:"
	@echo "   make dev-zoekt   # Zoekt on :6070"
	@echo "   make dev-api     # API on :8080"
	@echo "   make dev-indexer # Indexer worker"
	@echo "   make dev-web     # Frontend on :3000"

# =============================================================================
# Testing & Linting
# =============================================================================

test: ## Run tests
	$(GOCMD) test -v ./...

test-cover: ## Run tests with coverage
	$(GOCMD) test -v -cover -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

lint: ## Run linters
	golangci-lint run ./...

vet: ## Run go vet
	$(GOCMD) vet ./...

# =============================================================================
# Database
# =============================================================================

migrate-up: ## Run migrations
	$(DEV_ENV) $(GOCMD) run ./$(CMD_DIR)/migrate up

migrate-down: ## Rollback last migration
	$(DEV_ENV) $(GOCMD) run ./$(CMD_DIR)/migrate down

migrate-add: ## Run new migration (usage: make migrate-add name=your_migration_name)
	@echo "Creating new migration: $(name)"
	@if [ -z "$(name)" ]; then \
		echo "Error: Migration name is required. Usage: make migrate-add name=your_migration_name"; \
		exit 1; \
	fi
	$(DEV_ENV) $(GOCMD) run ./$(CMD_DIR)/migrate add $(name)

migrate-status: ## Show migration status
	$(DEV_ENV) $(GOCMD) run ./$(CMD_DIR)/migrate status

# =============================================================================
# Docker
# =============================================================================

docker-up: ## Start all services
	$(DOCKER_COMPOSE) up -d

docker-down: ## Stop all services
	$(DOCKER_COMPOSE) down

docker-logs: ## View logs (usage: make logs or make logs name=api)
	$(DOCKER_COMPOSE) logs -f $(name)

docker-ps: ## Show running containers
	$(DOCKER_COMPOSE) ps

docker-rebuild: ## Rebuild and restart services
	$(DOCKER_COMPOSE) down
	$(DOCKER_COMPOSE) up -d --build

docker-build: ## Build Docker images
	$(DOCKER_COMPOSE) build
	$(DOCKER) build -t code-search-zoekt-refresh:latest -f docker/zoekt-refresh.Dockerfile .

docker-push: ## Push Docker images to GitHub Container Registry
	@$(DOCKER) tag code-search-api:latest ghcr.io/techquestsdev/code-search-api:latest
	@$(DOCKER) tag code-search-indexer:latest ghcr.io/techquestsdev/code-search-indexer:latest
	@$(DOCKER) tag code-search-web:latest ghcr.io/techquestsdev/code-search-web:latest
	@$(DOCKER) tag code-search-website:latest ghcr.io/techquestsdev/code-search-website:latest
	@$(DOCKER) tag code-search-zoekt:latest ghcr.io/techquestsdev/code-search-zoekt:latest
	@$(DOCKER) tag code-search-zoekt-refresh:latest ghcr.io/techquestsdev/code-search-zoekt-refresh:latest
	@$(DOCKER) push ghcr.io/techquestsdev/code-search-api:latest
	@$(DOCKER) push ghcr.io/techquestsdev/code-search-indexer:latest
	@$(DOCKER) push ghcr.io/techquestsdev/code-search-web:latest
	@$(DOCKER) push ghcr.io/techquestsdev/code-search-website:latest
	@$(DOCKER) push ghcr.io/techquestsdev/code-search-zoekt:latest
	@$(DOCKER) push ghcr.io/techquestsdev/code-search-zoekt-refresh:latest
	@echo "✅ Docker images pushed to GitHub Container Registry."

# =============================================================================
# Cleanup
# =============================================================================

clean: ## Clean build artifacts
	rm -rf $(BIN_DIR) coverage.out coverage.html

docker-clean: ## Clean Docker build cache and unused images
	$(DOCKER) builder prune -f
	$(DOCKER) image prune -f

docker-prune: ## Full Docker cleanup (removes all unused data)
	$(DOCKER_COMPOSE) down -v --remove-orphans
	$(DOCKER) system prune -a -f --volumes
	@echo "✅ Cleaned. Run 'make up' to rebuild."

# =============================================================================
# Help
# =============================================================================

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'
