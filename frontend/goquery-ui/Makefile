# Simple Makefile for goquery-ui local development
# Focus: install deps, build bundle, watch, serve, docker setup

# configurable vars
NPM ?= npm
PORT ?= 8000
DIST_DIR := dist
WEBPACK := node_modules/.bin/webpack
# optional lock file (any one). If none exists we still allow install.
LOCKFILE := $(firstword $(wildcard package-lock.json npm-shrinkwrap.json pnpm-lock.yaml yarn.lock))
# OpenAPI spec version (can be overridden: `make VERSION=4.1.19 types`)
VERSION ?= 4.1.18
OPENAPI_SPEC := global-query_$(VERSION)_openapi.yaml
GENERATED_TYPES := src/api/generated.ts

# default target
.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make install        - install node dependencies"
	@echo "  make build          - build bundle -> $(DIST_DIR)/bundle.js"
	@echo "  make watch          - incremental rebuild (webpack --watch)"
	@echo "  make serve          - serve static files on http://localhost:$(PORT) (python simple server)"
	@echo "  make dev            - build then instructions to run watch+serve in 2 shells"
	@echo "  make clean          - remove dist directory"
	@echo "  make format         - format TypeScript/JavaScript files with prettier"
	@echo "  make generate-index - create a minimal index.html if missing"
	@echo "  make types          - fetch OpenAPI $(VERSION) and generate TS types ($(GENERATED_TYPES))"
	@echo "  make regen          - install + types + build"
	@echo "  make docker-dev     - docker compose dev profile (hot reload)"
	@echo "  make docker-stop    - stop docker compose dev profile"
	@echo "  make docker-logs    - view docker compose dev logs"
	@echo "  make docker-prod    - docker compose prod-like profile (Caddy)"
	@echo "  make docker-build   - build Caddy image locally"

# install: re-run if package.json or chosen lockfile changes
node_modules: package.json $(LOCKFILE)
	$(NPM) install
	@touch node_modules

.PHONY: install
install: node_modules

# fetch OpenAPI spec from GitHub releases if missing or version changed
$(OPENAPI_SPEC):
	@echo "Fetching OpenAPI spec version $(VERSION) ..."
	curl -fsSL -o "$@" "https://github.com/els0r/goProbe/releases/download/v$(VERSION)/global-query_$(VERSION)_openapi.yaml"
	@echo "Saved $@"

# openapi type generation (uses local dev dependency openapi-typescript)
$(GENERATED_TYPES): $(OPENAPI_SPEC) | install
	$(NPM) exec -- openapi-typescript "$(OPENAPI_SPEC)" --output "$(GENERATED_TYPES)"

.PHONY: types
types: $(GENERATED_TYPES)

.PHONY: regen
regen: install types build

# generate a minimal index.html (idempotent)
index.html:
	@[ -f index.html ] || echo '<!doctype html>\n<html>\n<head><meta charset="utf-8"><title>Goquery UI</title></head>\n<body>\n<div id="root"></div>\n<script src="dist/bundle.js"></script>\n</body>\n</html>' > index.html

.PHONY: generate-index
generate-index: index.html
	@echo "index.html present"

.PHONY: build
build: install index.html
	$(NPM) run build

.PHONY: watch
watch: install
	$(WEBPACK) --watch

# simple static file server (CTRL+C to stop)
.PHONY: serve
serve: build
	python3 -m http.server $(PORT)

.PHONY: dev
dev: build
	@echo "DEPRECATED: preferably use \"make docker-dev\""
	@echo "Run these in two terminals for live development:" \
	      "\n  Terminal 1: make watch" \
	      "\n  Terminal 2: make serve" \
	      "\nThen open http://localhost:$(PORT)"

.PHONY: clean
clean:
	rm -rf $(DIST_DIR) $(GENERATED_TYPES)

.PHONY: format
format: install
	$(NPM) exec -- prettier --write "src/**/*.{js,jsx,ts,tsx}"

.PHONY: docker-dev
docker-dev:
	@echo "Starting Docker development server with hot reload..."
	@echo "Access the application at http://localhost:5173"
	docker compose --profile dev up --build

.PHONY: docker-stop
docker-stop:
	docker compose --profile dev down

.PHONY: docker-logs
docker-logs:
	docker compose --profile dev logs -f

.PHONY: docker-prod
docker-prod: types
	docker compose --profile prod up --build

.PHONY: docker-build
docker-build: types
	docker build -t goquery-ui:caddy-local .
