SHELL := /bin/bash

APPS_DIR := apps
SIDECAR_DIR := $(APPS_DIR)/sidecar-core
DESKTOP_MANIFEST := $(APPS_DIR)/desktop/src-tauri/Cargo.toml
SIDECAR_BIN := $(APPS_DIR)/desktop/src-tauri/binaries/invest-compas-core

.PHONY: help
help:
	@echo "Invest Compass Make targets:"
	@echo "  make edit             Alias of make dev for one-command editing/development"
	@echo "  make dev              Start one-command local development"
	@echo "  make compile          Compile/check frontend, Rust backend, and Go sidecar"
	@echo "  make build            Build frontend, desktop backend, and Go sidecar"
	@echo "  make verify           Run format check, tests, checks, build, and diff check"
	@echo "  make frontend-build   Build React/Vite frontend"
	@echo "  make backend-build    Build Tauri/Rust desktop backend without bundling"
	@echo "  make go-build         Build Go sidecar binary"
	@echo "  make test             Run workspace tests"
	@echo "  make check            Run workspace checks"

.PHONY: install
install:
	pnpm --dir $(APPS_DIR) install

.PHONY: dev
dev:
	pnpm --dir $(APPS_DIR) dev

.PHONY: edit
edit: dev

.PHONY: compile
compile: frontend-check backend-check go-test

.PHONY: build
build: go-build frontend-build backend-build

.PHONY: verify
verify: format-check test check build diff-check

.PHONY: frontend-build
frontend-build:
	pnpm --dir $(APPS_DIR) --filter @invest-compass/frontend build

.PHONY: frontend-check
frontend-check:
	pnpm --dir $(APPS_DIR) --filter @invest-compass/frontend check

.PHONY: frontend-test
frontend-test:
	pnpm --dir $(APPS_DIR) --filter @invest-compass/frontend test

.PHONY: backend-build
backend-build:
	pnpm --dir $(APPS_DIR) --filter @invest-compass/desktop build

.PHONY: backend-check
backend-check:
	cargo check --manifest-path $(DESKTOP_MANIFEST)

.PHONY: backend-test
backend-test:
	cargo test --manifest-path $(DESKTOP_MANIFEST)

.PHONY: go-build
go-build:
	pnpm --dir $(APPS_DIR) sidecar:build

.PHONY: go-test
go-test:
	cd $(SIDECAR_DIR) && go test -tags sqlite_fts5 ./...

.PHONY: sidecar-build
sidecar-build: go-build

.PHONY: desktop-build
desktop-build: backend-build

.PHONY: test
test:
	pnpm --dir $(APPS_DIR) test

.PHONY: check
check:
	pnpm --dir $(APPS_DIR) check

.PHONY: format-check
format-check:
	pnpm --dir $(APPS_DIR) format:check

.PHONY: diff-check
diff-check:
	git diff --check
