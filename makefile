WEB_DIR := ./web
API_DIR := .
DEV_WEB_PORT ?= 5173
DEV_COMPOSE_FILE := docker-compose.dev.yml
DEV_POSTGRES_SERVICE := postgres
DEV_REDIS_SERVICE := redis
DEV_POSTGRES_DB := new_api
DEV_POSTGRES_USER := new_api
DEV_SQLITE_PATH ?= one-api.db

.PHONY: all build-web build-all-web start-api prepare-api-assets \
	infra-up infra-down infra-logs infra-reset dev dev-api dev-web \
	reset-setup test

all: build-all-web start-api

build-web:
	@echo "Building web frontend..."
	@cd $(WEB_DIR) && bun install --frozen-lockfile
	@cd $(WEB_DIR) && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$$(cat ../VERSION) bun run build

build-all-web: build-web

start-api:
	@echo "Starting api server..."
	@cd $(API_DIR) && go run main.go &

prepare-api-assets:
	@if [ ! -f "$(WEB_DIR)/dist/index.html" ]; then \
		echo "web/dist is required by Go embed; building it once before API startup..."; \
		$(MAKE) build-web; \
	fi

infra-up:
	@echo "Starting local PostgreSQL and Redis containers..."
	@docker compose -f $(DEV_COMPOSE_FILE) up -d --wait $(DEV_POSTGRES_SERVICE) $(DEV_REDIS_SERVICE)

infra-down:
	@echo "Stopping local PostgreSQL and Redis containers..."
	@docker compose -f $(DEV_COMPOSE_FILE) down

infra-logs:
	@docker compose -f $(DEV_COMPOSE_FILE) logs -f $(DEV_POSTGRES_SERVICE) $(DEV_REDIS_SERVICE)

infra-reset:
	@echo "Removing local PostgreSQL and Redis containers and data volumes..."
	@docker compose -f $(DEV_COMPOSE_FILE) down --volumes

dev-api: prepare-api-assets
	@if [ ! -f ".env" ]; then \
		echo "Missing .env. Copy .env.local.example to .env and replace SESSION_SECRET."; \
		exit 1; \
	fi
	@if grep -q '^SESSION_SECRET=replace-with-output-of-openssl-rand-hex-32$$' ".env"; then \
		echo "Refusing to start with the placeholder SESSION_SECRET. Generate one with: openssl rand -hex 32"; \
		exit 1; \
	fi
	@echo "Starting Go API on http://localhost:3000 (host process)..."
	@cd $(API_DIR) && go run .

dev-web:
	@echo "Starting Rsbuild web app on http://localhost:$(DEV_WEB_PORT) (host process)..."
	@cd $(WEB_DIR) && bun install --frozen-lockfile
	@cd $(WEB_DIR) && bun run dev -- --host 127.0.0.1 --port $(DEV_WEB_PORT)

dev:
	@echo "Local development uses three processes:"
	@echo "  1. make infra-up"
	@echo "  2. make dev-api    # run in a second terminal"
	@echo "  3. make dev-web    # run in a third terminal"
	@echo "The API and web app run on the host; Docker runs only PostgreSQL and Redis."

# The main package embeds the ignored web/dist output and is covered after build-web.
test:
	@echo "Testing root Go module..."
	@root_module=$$(GOWORK=off go list -m); \
		root_packages=$$(GOWORK=off go list -e ./... | grep -vxF "$$root_module"); \
		GOWORK=off go test $$root_packages
	@echo "Testing relaykit Go module..."
	@cd relaykit && GOWORK=off go test ./...

reset-setup:
	@echo "Resetting local setup wizard state..."
	@if docker compose -f $(DEV_COMPOSE_FILE) ps --services --status running | grep -qx "$(DEV_POSTGRES_SERVICE)"; then \
		echo "Detected running local PostgreSQL. Removing setup record and root users..."; \
		docker compose -f $(DEV_COMPOSE_FILE) exec -T $(DEV_POSTGRES_SERVICE) \
			psql -U $(DEV_POSTGRES_USER) -d $(DEV_POSTGRES_DB) \
			-c 'DELETE FROM setups;' \
			-c 'DELETE FROM users WHERE role = 100;' \
			-c "DELETE FROM options WHERE key IN ('SelfUseModeEnabled', 'DemoSiteEnabled');"; \
		echo "Setup state reset. Restart the local 'make dev-api' process."; \
	elif db_path="$${SQLITE_PATH:-$(DEV_SQLITE_PATH)}"; db_path="$${db_path%%\?*}"; [ -f "$$db_path" ]; then \
		db_path="$${SQLITE_PATH:-$(DEV_SQLITE_PATH)}"; \
		db_path="$${db_path%%\?*}"; \
		echo "Detected local SQLite database: $$db_path"; \
		sqlite3 "$$db_path" \
			"DELETE FROM setups; DELETE FROM users WHERE role = 100; DELETE FROM options WHERE key IN ('SelfUseModeEnabled', 'DemoSiteEnabled');"; \
		echo "SQLite setup state reset. Restart the local API process."; \
	else \
		echo "No running Docker dev PostgreSQL or local SQLite database found."; \
		echo "Start infrastructure with 'make infra-up', or set SQLITE_PATH/DEV_SQLITE_PATH."; \
		exit 1; \
	fi
