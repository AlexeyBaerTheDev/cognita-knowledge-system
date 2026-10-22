# ============================================================================
# Cognita — knowledge orchestrator
# ============================================================================

.PHONY: help up down restart build logs logs-% ps \
        migrate db-shell db-tables db-extensions \
        pull-models list-models test reset

include .env
export

.DEFAULT_GOAL := help

# ====== Help ======

help: ## Show available commands
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
		
# ====== Docker Compose ======

up: ## Bring up the entire stack
	docker compose up -d

down: ## Stop (data preserved)
	docker compose down

restart: ## Restart (down + up)
	docker compose down
	docker compose up -d

build: ## Rebuild api, orchestrator and worker images
	docker compose build api orchestrator worker

logs: ## Real-time logs for all services
	docker compose logs -f

logs-%: ## Logs for a specific service (make logs-api)
	docker compose logs -f $*

ps: ## Container status
	docker compose ps

# ====== Database ======

PSQL = docker compose exec -T database psql -v ON_ERROR_STOP=1 -U $(POSTGRES_USER) -d $(POSTGRES_DB)

migrate: ## Apply all migrations in order
	@for f in services/api/migrations/*.sql; do \
		echo "→ Applying $$f"; \
		$(PSQL) < $$f || exit 1; \
	done
	@echo "✓ Migrations applied."

db-shell: ## Open interactive psql session
	docker compose exec database psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

db-tables: ## List tables
	$(PSQL) -c "\dt"

db-extensions: ## List extensions (check pgvector)
	$(PSQL) -c "\dx"

# ====== Ollama ======

pull-models: ## Download embeddinggemma and qwen3:8b
	docker compose exec ollama ollama pull $(EMBEDDING_MODEL)
	docker compose exec ollama ollama pull $(LLM_MODEL)

list-models: ## Show downloaded models
	docker compose exec ollama ollama list

# ====== Tests ======

test: ## Run tests
	cd services/api && go test ./...
	cd services/orchestrator && ../../.venv/bin/python -m pytest

# ====== Api ======

run-server: ## Run api server locally (Postgres/Redis on localhost)
	cd services/api && \
		DATABASE_URL="postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:5432/$(POSTGRES_DB)?sslmode=disable" \
		REDIS_URL="redis://localhost:6379" \
		API_PORT=8080 go run ./cmd/server

# ====== Orchestrator ======

activate-venv: ## Activate .venv
	source .venv/bin/activate

run-worker: ## Run worker locally (Postgres/Redis/Ollama on localhost)
	cd services/orchestrator && \
		DATABASE_URL="postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:5432/$(POSTGRES_DB)?sslmode=disable" \
		REDIS_URL="redis://localhost:6379" \
		OLLAMA_URL="http://localhost:11434" \
		../../.venv/bin/python -m app.worker

# ====== Danger ======

reset: ## ⚠ Delete all containers AND volumes (including Ollama models)
	@echo "⚠  This will delete ALL data: Postgres, Redis, Ollama models."
	@read -p "Continue? [y/N] " ans; \
		if [ "$$ans" != "y" ]; then echo "Canceled."; exit 1; fi
	docker compose down -v
	docker compose up -d