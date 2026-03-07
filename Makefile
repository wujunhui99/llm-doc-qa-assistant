SHELL := /bin/bash

.DEFAULT_GOAL := help

PORT_CORE := 19090
PORT_LLM := 51000
PORT_API := 8080
PORT_FRONTEND := 5173

LOG_DIR := logs
AGENT_PID_FILE := $(LOG_DIR)/agent-python-rpc.pid
CORE_PID_FILE := $(LOG_DIR)/core-go-rpc.pid
API_PID_FILE := $(LOG_DIR)/api-go.pid
FRONTEND_PID_FILE := $(LOG_DIR)/frontend.pid

.PHONY: help start stop restart status _dispatch _start_llm _start_core _start_api _start_frontend _stop_llm _stop_core _stop_api _stop_frontend _status_llm _status_core _status_api _status_frontend
.PHONY: all agent agent-python-rpc llm llm-python-rpc py-rpc core core-go-rpc go-rpc api api-go frontend fe web

help:
	@echo "Usage:"
	@echo "  make start <service|all>"
	@echo "  make stop <service|all>"
	@echo "  make restart <service|all>"
	@echo "  make status <service|all>"
	@echo ""
	@echo "Services:"
	@echo "  agent (aliases: agent-python-rpc, llm, llm-python-rpc, py-rpc)"
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

status:
	@svc="$(word 2,$(MAKECMDGOALS))"; \
	if [ -z "$$svc" ]; then svc="all"; fi; \
	$(MAKE) --no-print-directory _dispatch ACTION=status SERVICE=$$svc

_dispatch:
	@set -euo pipefail; \
	action="$(ACTION)"; \
	service="$(SERVICE)"; \
	if [ -z "$$service" ]; then service="all"; fi; \
	case "$$service" in \
		all) \
			if [ "$$action" = "stop" ]; then list="frontend api core llm"; else list="llm core api frontend"; fi ;; \
		agent|agent-python-rpc|llm|llm-python-rpc|py-rpc) list="llm" ;; \
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
	@if [ -f "$(AGENT_PID_FILE)" ]; then \
		pid="$$(cat $(AGENT_PID_FILE) 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
			pids="$$(lsof -tiTCP:$(PORT_LLM) -sTCP:LISTEN 2>/dev/null || true)"; \
			if [ -n "$$pids" ]; then \
				echo "[agent] already running (pid: $$pid, port: $(PORT_LLM))"; \
				exit 0; \
			fi; \
			echo "[agent] stale pid $$pid without listener, cleaning up"; \
			kill "$$pid" 2>/dev/null || true; \
			sleep 1; \
			kill -9 "$$pid" 2>/dev/null || true; \
		fi; \
		rm -f "$(AGENT_PID_FILE)"; \
	fi
	@port=$(PORT_LLM); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[agent] already running on :$$port (pid: $$pids)"; exit 0; fi; \
	echo "[agent] starting on :$$port"; \
	nohup bash -lc 'cd backend/apps/agent-python-rpc && AGENT_RPC_HOST=127.0.0.1 AGENT_RPC_PORT=$(PORT_LLM) LLM_RPC_HOST=127.0.0.1 LLM_RPC_PORT=$(PORT_LLM) exec python3 -m app.server' > $(LOG_DIR)/agent-python-rpc.log 2>&1 < /dev/null & echo $$! > $(AGENT_PID_FILE); \
	for i in {1..15}; do \
		pid="$$(cat $(AGENT_PID_FILE) 2>/dev/null || true)"; \
		pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null && [ -n "$$pids" ]; then \
			echo "[agent] started (pid: $$pid)"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "[agent] failed to start, check $(LOG_DIR)/agent-python-rpc.log"; \
	rm -f "$(AGENT_PID_FILE)"; \
	tail -n 50 $(LOG_DIR)/agent-python-rpc.log || true; \
	exit 1

_start_core:
	@mkdir -p $(LOG_DIR)
	@if [ -f "$(CORE_PID_FILE)" ]; then \
		pid="$$(cat $(CORE_PID_FILE) 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
			pids="$$(lsof -tiTCP:$(PORT_CORE) -sTCP:LISTEN 2>/dev/null || true)"; \
			if [ -n "$$pids" ]; then \
				echo "[core] already running (pid: $$pid, port: $(PORT_CORE))"; \
				exit 0; \
			fi; \
			echo "[core] stale pid $$pid without listener, cleaning up"; \
			kill "$$pid" 2>/dev/null || true; \
			sleep 1; \
			kill -9 "$$pid" 2>/dev/null || true; \
		fi; \
		rm -f "$(CORE_PID_FILE)"; \
	fi
	@port=$(PORT_CORE); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[core] already running on :$$port (pid: $$pids)"; exit 0; fi; \
	echo "[core] starting on :$$port"; \
	nohup bash -lc 'cd backend && AGENT_RPC_ADDR=127.0.0.1:$(PORT_LLM) LLM_RPC_ADDR=127.0.0.1:$(PORT_LLM) exec go run ./apps/core-go-rpc/cmd/server' > $(LOG_DIR)/core-go-rpc.log 2>&1 < /dev/null & echo $$! > $(CORE_PID_FILE); \
	for i in {1..20}; do \
		pid="$$(cat $(CORE_PID_FILE) 2>/dev/null || true)"; \
		pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null && [ -n "$$pids" ]; then \
			echo "[core] started (pid: $$pid)"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "[core] failed to start, check $(LOG_DIR)/core-go-rpc.log"; \
	rm -f "$(CORE_PID_FILE)"; \
	tail -n 50 $(LOG_DIR)/core-go-rpc.log || true; \
	exit 1

_start_api:
	@mkdir -p $(LOG_DIR)
	@if [ -f "$(API_PID_FILE)" ]; then \
		pid="$$(cat $(API_PID_FILE) 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
			pids="$$(lsof -tiTCP:$(PORT_API) -sTCP:LISTEN 2>/dev/null || true)"; \
			if [ -n "$$pids" ]; then \
				echo "[api] already running (pid: $$pid, port: $(PORT_API))"; \
				exit 0; \
			fi; \
			echo "[api] stale pid $$pid without listener, cleaning up"; \
			kill "$$pid" 2>/dev/null || true; \
			sleep 1; \
			kill -9 "$$pid" 2>/dev/null || true; \
		fi; \
		rm -f "$(API_PID_FILE)"; \
	fi
	@port=$(PORT_API); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[api] already running on :$$port (pid: $$pids)"; exit 0; fi; \
	echo "[api] starting on :$$port"; \
	nohup bash -lc 'cd backend && CORE_RPC_ADDR=127.0.0.1:19090 PORT=8080 exec go run ./apps/api-go/cmd/api' > $(LOG_DIR)/api-go.log 2>&1 < /dev/null & echo $$! > $(API_PID_FILE); \
	for i in {1..20}; do \
		pid="$$(cat $(API_PID_FILE) 2>/dev/null || true)"; \
		pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null && [ -n "$$pids" ]; then \
			echo "[api] started (pid: $$pid)"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "[api] failed to start, check $(LOG_DIR)/api-go.log"; \
	rm -f "$(API_PID_FILE)"; \
	tail -n 50 $(LOG_DIR)/api-go.log || true; \
	exit 1

_start_frontend:
	@mkdir -p $(LOG_DIR)
	@if [ -f "$(FRONTEND_PID_FILE)" ]; then \
		pid="$$(cat $(FRONTEND_PID_FILE) 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
			pids="$$(lsof -tiTCP:$(PORT_FRONTEND) -sTCP:LISTEN 2>/dev/null || true)"; \
			if [ -n "$$pids" ]; then \
				echo "[frontend] already running (pid: $$pid, port: $(PORT_FRONTEND))"; \
				exit 0; \
			fi; \
			echo "[frontend] stale pid $$pid without listener, cleaning up"; \
			kill "$$pid" 2>/dev/null || true; \
			sleep 1; \
			kill -9 "$$pid" 2>/dev/null || true; \
		fi; \
		rm -f "$(FRONTEND_PID_FILE)"; \
	fi
	@port=$(PORT_FRONTEND); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[frontend] already running on :$$port (pid: $$pids)"; exit 0; fi; \
	echo "[frontend] starting on :$$port"; \
	nohup bash -lc 'cd frontend && exec npm run dev -- --host 0.0.0.0 --port 5173' > $(LOG_DIR)/frontend.log 2>&1 < /dev/null & echo $$! > $(FRONTEND_PID_FILE); \
	for i in {1..20}; do \
		pid="$$(cat $(FRONTEND_PID_FILE) 2>/dev/null || true)"; \
		pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null && [ -n "$$pids" ]; then \
			echo "[frontend] started (pid: $$pid)"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "[frontend] failed to start, check $(LOG_DIR)/frontend.log"; \
	rm -f "$(FRONTEND_PID_FILE)"; \
	tail -n 50 $(LOG_DIR)/frontend.log || true; \
	exit 1

_stop_llm:
	@pids=""; \
	if [ -f "$(AGENT_PID_FILE)" ]; then \
		pid="$$(cat $(AGENT_PID_FILE) 2>/dev/null || true)"; \
		if [ -n "$$pid" ]; then pids="$$pid"; fi; \
	fi; \
	if [ -z "$$pids" ]; then \
		pids="$$(pgrep -f 'backend/apps/agent-python-rpc.*app.server' 2>/dev/null || true)"; \
	fi; \
	if [ -z "$$pids" ]; then echo "[agent] not running"; rm -f "$(AGENT_PID_FILE)"; exit 0; fi; \
	echo "[agent] stopping pid: $$pids"; \
	echo "$$pids" | xargs kill 2>/dev/null || true; \
	sleep 1; \
	left="$$(echo "$$pids" | xargs -I{} sh -c 'kill -0 {} 2>/dev/null && echo {}' || true)"; \
	if [ -n "$$left" ]; then echo "$$left" | xargs kill -9 2>/dev/null || true; fi; \
	rm -f "$(AGENT_PID_FILE)"; \
	echo "[agent] stopped"

_stop_core:
	@pids=""; \
	if [ -f "$(CORE_PID_FILE)" ]; then \
		pid="$$(cat $(CORE_PID_FILE) 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then pids="$$pid"; fi; \
	fi; \
	if [ -z "$$pids" ]; then \
		pids="$$(lsof -tiTCP:$(PORT_CORE) -sTCP:LISTEN 2>/dev/null || true)"; \
	fi; \
	if [ -z "$$pids" ]; then echo "[core] not running on :$(PORT_CORE)"; rm -f "$(CORE_PID_FILE)"; exit 0; fi; \
	echo "[core] stopping pid: $$pids"; \
	echo "$$pids" | xargs kill 2>/dev/null || true; \
	sleep 1; \
	left="$$(echo "$$pids" | xargs -I{} sh -c 'kill -0 {} 2>/dev/null && echo {}' || true)"; \
	if [ -n "$$left" ]; then echo "$$left" | xargs kill -9 2>/dev/null || true; fi; \
	left_port="$$(lsof -tiTCP:$(PORT_CORE) -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$left_port" ]; then echo "$$left_port" | xargs kill -9 2>/dev/null || true; fi; \
	rm -f "$(CORE_PID_FILE)"; \
	echo "[core] stopped"

_stop_api:
	@pids=""; \
	if [ -f "$(API_PID_FILE)" ]; then \
		pid="$$(cat $(API_PID_FILE) 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then pids="$$pid"; fi; \
	fi; \
	if [ -z "$$pids" ]; then \
		pids="$$(lsof -tiTCP:$(PORT_API) -sTCP:LISTEN 2>/dev/null || true)"; \
	fi; \
	if [ -z "$$pids" ]; then echo "[api] not running on :$(PORT_API)"; rm -f "$(API_PID_FILE)"; exit 0; fi; \
	echo "[api] stopping pid: $$pids"; \
	echo "$$pids" | xargs kill 2>/dev/null || true; \
	sleep 1; \
	left="$$(echo "$$pids" | xargs -I{} sh -c 'kill -0 {} 2>/dev/null && echo {}' || true)"; \
	if [ -n "$$left" ]; then echo "$$left" | xargs kill -9 2>/dev/null || true; fi; \
	left_port="$$(lsof -tiTCP:$(PORT_API) -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$left_port" ]; then echo "$$left_port" | xargs kill -9 2>/dev/null || true; fi; \
	rm -f "$(API_PID_FILE)"; \
	echo "[api] stopped"

_stop_frontend:
	@pids=""; \
	if [ -f "$(FRONTEND_PID_FILE)" ]; then \
		pid="$$(cat $(FRONTEND_PID_FILE) 2>/dev/null || true)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then pids="$$pid"; fi; \
	fi; \
	if [ -z "$$pids" ]; then \
		pids="$$(lsof -tiTCP:$(PORT_FRONTEND) -sTCP:LISTEN 2>/dev/null || true)"; \
	fi; \
	if [ -z "$$pids" ]; then echo "[frontend] not running on :$(PORT_FRONTEND)"; rm -f "$(FRONTEND_PID_FILE)"; exit 0; fi; \
	echo "[frontend] stopping pid: $$pids"; \
	echo "$$pids" | xargs kill 2>/dev/null || true; \
	sleep 1; \
	left="$$(echo "$$pids" | xargs -I{} sh -c 'kill -0 {} 2>/dev/null && echo {}' || true)"; \
	if [ -n "$$left" ]; then echo "$$left" | xargs kill -9 2>/dev/null || true; fi; \
	left_port="$$(lsof -tiTCP:$(PORT_FRONTEND) -sTCP:LISTEN 2>/dev/null || true)"; \
	if [ -n "$$left_port" ]; then echo "$$left_port" | xargs kill -9 2>/dev/null || true; fi; \
	rm -f "$(FRONTEND_PID_FILE)"; \
	echo "[frontend] stopped"

_status_llm:
	@port=$(PORT_LLM); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	pid_file="$$(cat $(AGENT_PID_FILE) 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[agent] running on :$$port (listen pid: $$pids, pidfile: $$pid_file)"; else echo "[agent] stopped (pidfile: $$pid_file)"; fi

_status_core:
	@port=$(PORT_CORE); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	pid_file="$$(cat $(CORE_PID_FILE) 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[core] running on :$$port (listen pid: $$pids, pidfile: $$pid_file)"; else echo "[core] stopped (pidfile: $$pid_file)"; fi

_status_api:
	@port=$(PORT_API); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	pid_file="$$(cat $(API_PID_FILE) 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[api] running on :$$port (listen pid: $$pids, pidfile: $$pid_file)"; else echo "[api] stopped (pidfile: $$pid_file)"; fi

_status_frontend:
	@port=$(PORT_FRONTEND); \
	pids="$$(lsof -tiTCP:$$port -sTCP:LISTEN 2>/dev/null || true)"; \
	pid_file="$$(cat $(FRONTEND_PID_FILE) 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then echo "[frontend] running on :$$port (listen pid: $$pids, pidfile: $$pid_file)"; else echo "[frontend] stopped (pidfile: $$pid_file)"; fi

%:
	@:

all agent agent-python-rpc llm llm-python-rpc py-rpc core core-go-rpc go-rpc api api-go frontend fe web:
	@:
