# Mattermost local development helpers.
#
# Start/stop the app (Go server) and the web client (webapp) without having
# to remember the two sub-Makefiles.
#
#   make start       app + web, in the background
#   make status      what is running, and where
#   make stop        stop web + app (docker services stay up)
#   make stop-all    also stop the docker services (postgres, redis, ...)
#
# The web client can run in two modes:
#   make start-web                     webpack --watch, served by the server on :8065
#   make start-web WEB_MODE=dev-server webpack-dev-server + HMR on :9005
#
# Logs and pid files live in logs/, which is gitignored.

# The webapp pins Node/npm exactly (webapp/.npmrc sets engine-strict=true), so a
# locally installed toolchain takes precedence when present.
NODE_HOME ?= $(HOME)/.local/share/mattermost/node
ifneq (,$(wildcard $(NODE_HOME)/bin/node))
export PATH := $(NODE_HOME)/bin:$(PATH)
endif

# Dev configuration (database, OIDC, admin credentials, ...). Sourced with
# `set -a` so every value reaches the server and the sub-Makefiles.
ENV_FILE ?= .env
LOAD_ENV = if [ -f $(ENV_FILE) ]; then set -a; . ./$(ENV_FILE); set +a; fi

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.ONESHELL:
MAKEFLAGS += --no-print-directory
.DEFAULT_GOAL := help

SERVER_DIR := server
WEBAPP_DIR := webapp
LOG_DIR := logs
APP_LOG := $(LOG_DIR)/app.log
WEB_LOG := $(LOG_DIR)/web.log
WEB_PID := $(LOG_DIR)/web.pid
WEB_MODE_FILE := $(LOG_DIR)/web.mode

APP_URL := http://localhost:8065
WEB_DEV_URL := http://localhost:9005

# Agents plugin source checkout used by `make plugin-install`.
PLUGIN_DIR ?= ../mattermost-plugin-agents
PLUGIN_GIT_URL ?= https://github.com/r2d-ai/mattermost-plugin-agents.git

# watch: webpack --watch, assets served by the Go server (port 8065)
# dev-server: webpack-dev-server with HMR (port 9005, proxies /api to 8065)
WEB_MODE ?= watch
ifeq ($(WEB_MODE),dev-server)
WEB_TARGET := dev
else
WEB_TARGET := run
endif

.PHONY: help start stop restart start-app stop-app restart-app start-web stop-web restart-web status logs logs-app logs-web stop-all deps admin plugin-install

help: ## Show this help
	@printf 'Mattermost dev targets:\n\n'
	grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	printf '\nWEB_MODE=dev-server switches the web client to webpack-dev-server on port 9005.\n'

start: ## Start app + web in the background
	@$(MAKE) start-app
	$(MAKE) start-web

stop: ## Stop web + app (docker services keep running)
	@$(MAKE) stop-web
	$(MAKE) stop-app

restart: ## Restart web + app
	@$(MAKE) stop
	$(MAKE) start

start-app: ## Start the app (Go server) in the background
	@mkdir -p $(LOG_DIR)
	$(LOAD_ENV)
	if curl -sf -o /dev/null --max-time 2 $(APP_URL)/api/v4/system/ping || ps -ef | grep '[g]o run' | grep -q mattermost; then
		echo "App is already running -> $(APP_URL)"
		exit 0
	fi
	: > $(APP_LOG)
	echo "Starting app -> $(APP_URL) (the first build can take a few minutes)"
	echo "  log: $(APP_LOG)"
	$(MAKE) -C $(SERVER_DIR) run-server >> $(APP_LOG) 2>&1
	echo "App started. Follow logs with: make logs-app"

stop-app: ## Stop the app (Go server)
	@echo "Stopping app"
	$(MAKE) -C $(SERVER_DIR) stop-server

restart-app: ## Restart the app (Go server)
	$(MAKE) stop-app
	$(MAKE) start-app

deps: ## Start the docker services only (postgres, redis, inbucket, ...)
	@$(LOAD_ENV)
	$(MAKE) -C $(SERVER_DIR) start-docker

admin: ## Create the local admin user (the app must be running)
	@$(LOAD_ENV)
	$(MAKE) -C $(SERVER_DIR) mmctl-build
	if ! $(SERVER_DIR)/bin/mmctl --local user create \
		--email "$${DEV_ADMIN_EMAIL}" \
		--username "$${DEV_ADMIN_USERNAME}" \
		--password "$${DEV_ADMIN_PASSWORD}" \
		--email-verified \
		--system-admin; then
		echo "Admin user was not created (it may already exist)."
	fi

plugin-install: ## Build and deploy the Agents plugin (the app must be running)
	@$(LOAD_ENV)
	if [ ! -d "$(PLUGIN_DIR)" ]; then
		echo "Cloning $(PLUGIN_GIT_URL) into $(PLUGIN_DIR)"
		git clone $(PLUGIN_GIT_URL) $(PLUGIN_DIR)
	fi
	echo "Building and deploying the Agents plugin from $(PLUGIN_DIR)"
	MM_ADMIN_USERNAME="$${DEV_ADMIN_USERNAME}" \
	MM_ADMIN_PASSWORD="$${DEV_ADMIN_PASSWORD}" \
	MM_SERVICESETTINGS_SITEURL="$${MM_SERVICESETTINGS_SITEURL}" \
	$(MAKE) -C $(PLUGIN_DIR) deploy MM_SERVICESETTINGS_ENABLEDEVELOPER=true

start-web: ## Start the web client in the background (WEB_MODE=watch|dev-server)
	@mkdir -p $(LOG_DIR)
	$(LOAD_ENV)
	if [ -f $(WEB_PID) ] && kill -0 "$$(cat $(WEB_PID))" 2>/dev/null; then
		echo "Web is already running (pid $$(cat $(WEB_PID)))"
		exit 0
	fi
	rm -f $(WEB_PID)
	if [ ! -d $(WEBAPP_DIR)/node_modules ]; then
		echo "Installing webapp dependencies first; this can take a while..."
	fi
	echo "Starting web (mode: $(WEB_MODE))"
	echo "  log: $(WEB_LOG)"
	( cd $(WEBAPP_DIR) && exec nohup $(MAKE) $(WEB_TARGET) > $(abspath $(WEB_LOG)) 2>&1 < /dev/null ) &
	echo $$! > $(WEB_PID)
	echo "$(WEB_MODE)" > $(WEB_MODE_FILE)
	echo "Web started (pid $$(cat $(WEB_PID))). Follow logs with: make logs-web"

stop-web: ## Stop the web client
	@if [ ! -f $(WEB_PID) ]; then
		echo "Web is not running"
		exit 0
	fi
	PID="$$(cat $(WEB_PID))"
	rm -f $(WEB_PID) $(WEB_MODE_FILE)
	if ! kill -0 "$$PID" 2>/dev/null; then
		echo "Web is not running (removed stale pid file)"
		exit 0
	fi
	echo "Stopping web (pid $$PID)"
	PIDS="$$PID"
	FRONTIER="$$PID"
	while [ -n "$$FRONTIER" ]; do
		NEXT=""
		for P in $$FRONTIER; do
			NEXT="$$NEXT $$(pgrep -P "$$P" 2>/dev/null || true)"
		done
		if [ -n "$$NEXT" ]; then
			PIDS="$$PIDS $$NEXT"
		fi
		FRONTIER="$$NEXT"
	done
	kill -TERM $$PIDS 2>/dev/null || true
	ALIVE=""
	for _ in 1 2 3 4 5; do
		ALIVE=""
		for P in $$PIDS; do
			if kill -0 "$$P" 2>/dev/null; then
				ALIVE="$$ALIVE $$P"
			fi
		done
		if [ -z "$$ALIVE" ]; then
			break
		fi
		sleep 1
	done
	if [ -n "$$ALIVE" ]; then
		kill -KILL $$ALIVE 2>/dev/null || true
	fi
	echo "Web stopped"

restart-web: ## Restart the web client
	@$(MAKE) stop-web
	$(MAKE) start-web

status: ## Show whether app and web are running
	@$(LOAD_ENV)
	SITE_URL="$${MM_SERVICESETTINGS_SITEURL:-$(APP_URL)}"
	if curl -sf -o /dev/null --max-time 2 "$$SITE_URL/api/v4/system/ping"; then
		echo "app: running  $$SITE_URL"
	else
		echo "app: stopped  $$SITE_URL"
	fi
	MODE="$$(cat $(WEB_MODE_FILE) 2>/dev/null || echo '$(WEB_MODE)')"
	if [ "$$MODE" = "dev-server" ]; then
		URL="$(WEB_DEV_URL)"
	else
		URL="$$SITE_URL"
	fi
	if [ -f $(WEB_PID) ] && kill -0 "$$(cat $(WEB_PID))" 2>/dev/null; then
		echo "web: running  $$URL (pid $$(cat $(WEB_PID)), mode $$MODE)"
	else
		echo "web: stopped  (last mode: $$MODE)"
	fi

logs: ## Follow app and web logs
	@mkdir -p $(LOG_DIR)
	touch $(APP_LOG) $(WEB_LOG)
	tail -n 50 -f $(APP_LOG) $(WEB_LOG)

logs-app: ## Follow the app log
	@mkdir -p $(LOG_DIR)
	touch $(APP_LOG)
	tail -n 50 -f $(APP_LOG)

logs-web: ## Follow the web log
	@mkdir -p $(LOG_DIR)
	touch $(WEB_LOG)
	tail -n 50 -f $(WEB_LOG)

stop-all: ## Stop web, app and the docker services
	@$(MAKE) stop
	$(MAKE) -C $(SERVER_DIR) stop-docker
