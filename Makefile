SHELL := /bin/bash

.DEFAULT_GOAL := help

PORT_CORE := 19090
PORT_LLM := 51000
PORT_API := 8080
PORT_FRONTEND := 5173

LOG_DIR := logs
LLM_PID_FILE := $(LOG_DIR)/llm-python-rpc.pid

.PHONY: help start stop restart _dispatch _start_llm _start_core _start_api _start_frontend _stop_llm _stop_core _stop_api _stop_frontend
.PHONY: all llm llm-python-rpc py-rpc core core-go-rpc go-rpc api api-go frontend fe web

help:
	@echo "Usage:"
	@echo "  make start <service|all>"
	@echo "  make stop <service|all>"
	@echo "  make restart <service|all>"
	@echo ""
	@echo "Services:"
	@echo "  llm (aliases: llm-python-rpc, py-rpc)"
	@echo "  core (aliases: core-go-rpc, go-rpc)"
	@echo "  api (aliases: api-go)"
	@echo "  frontend (aliases: fe, web)"
	@echo "  all"

start:
	@svc="$(word 2,$(MAKECMDGOALS))"; \
	if [ -z "$$svc" ]; then svc="all"; fi; \
	$(MAKE) --no-print-directory _dispatch ACTION=start SERVICE=$$svc

stop:
	@svc="$(word 2,$(MAKECMDGOALS))"; \
	if [ -z "$$svc" ]; then svc="all"; fi; \
	$(MAKE) --no-print-directory _dispatch ACTION=stop SERVICE=$$svc

restart:
	@svc="$(word 2,$(MAKECMDGOALS))"; \
	if [ -z "$$svc" ]; then svc="all"; fi; \
	$(MAKE) --no-print-directory _dispatch ACTION=stop SERVICE=$$svc; \
	$(MAKE) --no-print-directory _dispatch ACTION=start SERVICE=$$svc

_dispatch:
	@set -euo pipefail; \
	action="$(ACTION)"; \
	service="$(SERVICE)"; \
	if [ -z "$$service" ]; then service="all"; fi; \
	case "$$service" in \
		all) \
			if [ "$$action" = "stop" ]; then list="frontend api core llm"; else list="llm core api frontend"; fi ;; \
		llm|llm-python-rpc|py-rpc) list="llm" ;; \
		core|core-go-rpc|go-rpc) list="core" ;; \
		api|api-go) list="api" ;; \
		frontend|fe|web) list="frontend" ;; \
		*) echo "Unknown service: $$service"; exit 1 ;; \
	esac; \
	for s in $$list; do \
		$(MAKE) --no-print-directory _$${action}_$$s; \
	done

_start_llm:
	@mkdir -p $(LOG_DIR)
	@if [ -f "$(LLM_PID_FILE)" ]; then \
		pid="$$(cat $(LLM_PID_FILE) 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
			echo "[llm] already running (pid: $$pid, port: $(PORT_LLM))"; \
			exit 0; \
		fi; \
		rm -f "$(LLM_PID_FILE)"; \
	fi
	@port=$(PORT_LLM); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[llm] already running on :$$port (pid: $$pids)"; exit 0; fi; \
	echo "[llm] starting on :$$port"; \
	nohup bash -lc 'cd backend/apps/llm-python-rpc && LLM_RPC_HOST=127.0.0.1 LLM_RPC_PORT=$(PORT_LLM) python3 -m app.server' > $(LOG_DIR)/llm-python-rpc.log 2>&1 & echo $$! > $(LLM_PID_FILE); \
	sleep 2; \
	pid="$$(cat $(LLM_PID_FILE) 2>/dev/null || true)"; \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -z "$$pid" ] || ! kill -0 "$$pid" 2>/dev/null || [ -z "$$pids" ]; then \
		echo "[llm] failed to start, check $(LOG_DIR)/llm-python-rpc.log"; \
		rm -f "$(LLM_PID_FILE)"; \
		tail -n 30 $(LOG_DIR)/llm-python-rpc.log || true; \
		exit 1; \
	fi; \
	echo "[llm] started (pid: $$pid)"

_start_core:
	@mkdir -p $(LOG_DIR)
	@port=$(PORT_CORE); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[core] already running on :$$port (pid: $$pids)"; exit 0; fi; \
	echo "[core] starting on :$$port"; \
	nohup bash -lc 'cd backend && LLM_RPC_ADDR=127.0.0.1:$(PORT_LLM) go run ./apps/core-go-rpc/cmd/server' > $(LOG_DIR)/core-go-rpc.log 2>&1 & \
	sleep 2; \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -z "$$pids" ]; then \
		echo "[core] failed to start, check $(LOG_DIR)/core-go-rpc.log"; \
		tail -n 30 $(LOG_DIR)/core-go-rpc.log || true; \
		exit 1; \
	fi; \
	echo "[core] started (pid: $$pids)"

_start_api:
	@mkdir -p $(LOG_DIR)
	@port=$(PORT_API); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[api] already running on :$$port (pid: $$pids)"; exit 0; fi; \
	echo "[api] starting on :$$port"; \
	nohup bash -lc 'cd backend && CORE_RPC_ADDR=127.0.0.1:19090 PORT=8080 go run ./apps/api-go/cmd/api' > $(LOG_DIR)/api-go.log 2>&1 & \
	sleep 2; \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -z "$$pids" ]; then \
		echo "[api] failed to start, check $(LOG_DIR)/api-go.log"; \
		tail -n 30 $(LOG_DIR)/api-go.log || true; \
		exit 1; \
	fi; \
	echo "[api] started (pid: $$pids)"

_start_frontend:
	@mkdir -p $(LOG_DIR)
	@port=$(PORT_FRONTEND); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[frontend] already running on :$$port (pid: $$pids)"; exit 0; fi; \
	echo "[frontend] starting on :$$port"; \
	nohup bash -lc 'cd frontend && npm run dev -- --host 0.0.0.0 --port 5173' > $(LOG_DIR)/frontend.log 2>&1 & \
	sleep 2; \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -z "$$pids" ]; then \
		echo "[frontend] failed to start, check $(LOG_DIR)/frontend.log"; \
		tail -n 30 $(LOG_DIR)/frontend.log || true; \
		exit 1; \
	fi; \
	echo "[frontend] started (pid: $$pids)"

_stop_llm:
	@pids=""; \
	if [ -f "$(LLM_PID_FILE)" ]; then \
		pid="$$(cat $(LLM_PID_FILE) 2>/dev/null || true)"; \
		if [ -n "$$pid" ]; then pids="$$pid"; fi; \
	fi; \
	if [ -z "$$pids" ]; then \
		pids="$$(pgrep -f 'backend/apps/llm-python-rpc.*app.server' 2>/dev/null || true)"; \
	fi; \
	if [ -z "$$pids" ]; then echo "[llm] not running"; rm -f "$(LLM_PID_FILE)"; exit 0; fi; \
	echo "[llm] stopping pid: $$pids"; \
	echo "$$pids" | xargs kill; \
	sleep 1; \
	left="$$(echo "$$pids" | xargs -I{} sh -c 'kill -0 {} 2>/dev/null && echo {}' || true)"; \
	if [ -n "$$left" ]; then echo "$$left" | xargs kill -9; fi; \
	rm -f "$(LLM_PID_FILE)"; \
	echo "[llm] stopped"

_stop_core:
	@port=$(PORT_CORE); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -z "$$pids" ]; then echo "[core] not running on :$$port"; exit 0; fi; \
	echo "[core] stopping pid: $$pids"; \
	echo "$$pids" | xargs kill; \
	sleep 1; \
	left="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$left" ]; then echo "$$left" | xargs kill -9; fi; \
	echo "[core] stopped"

_stop_api:
	@port=$(PORT_API); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -z "$$pids" ]; then echo "[api] not running on :$$port"; exit 0; fi; \
	echo "[api] stopping pid: $$pids"; \
	echo "$$pids" | xargs kill; \
	sleep 1; \
	left="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$left" ]; then echo "$$left" | xargs kill -9; fi; \
	echo "[api] stopped"

_stop_frontend:
	@port=$(PORT_FRONTEND); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -z "$$pids" ]; then echo "[frontend] not running on :$$port"; exit 0; fi; \
	echo "[frontend] stopping pid: $$pids"; \
	echo "$$pids" | xargs kill; \
	sleep 1; \
	left="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$left" ]; then echo "$$left" | xargs kill -9; fi; \
	echo "[frontend] stopped"

%:
	@:

all llm llm-python-rpc py-rpc core core-go-rpc go-rpc api api-go frontend fe web:
	@:
