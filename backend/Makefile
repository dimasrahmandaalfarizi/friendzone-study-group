.DEFAULT_GOAL := help

GO ?= go
ifeq ($(OS),Windows_NT)
SWAG ?= $(shell $(GO) env GOPATH)/bin/swag.exe
BINARY_NAME := friendzone-api.exe
MKDIR_BIN = powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(BINARY_DIR)' | Out-Null"
CLEAN_BIN = powershell -NoProfile -Command "if (Test-Path '$(BINARY_DIR)') { Remove-Item -Recurse -Force '$(BINARY_DIR)' }"
else
SWAG ?= $(shell $(GO) env GOPATH)/bin/swag
BINARY_NAME := friendzone-api
MKDIR_BIN = mkdir -p $(BINARY_DIR)
CLEAN_BIN = rm -rf $(BINARY_DIR)
endif
MAIN_PACKAGE := ./cmd/api
BINARY_DIR := bin

.PHONY: help init setup deps tidy install-swag docs run dev build test vet fmt clean check-env

help:
	@echo Friendzone development commands:
	@echo   make init          Install dependencies and Swagger CLI, then generate docs
	@echo   make setup         Alias for make init
	@echo   make deps          Download Go modules
	@echo   make install-swag  Install the swag CLI
	@echo   make docs          Update Swagger docs
	@echo   make run           Update Swagger docs and run the API
	@echo   make dev           Alias for make run
	@echo   make build         Build the API executable
	@echo   make test          Run all Go tests
	@echo   make vet           Run go vet
	@echo   make fmt           Format Go files
	@echo   make check-env     Check required environment variables
	@echo   make clean         Remove generated binaries

init: deps install-swag docs

setup: init

deps:
	$(GO) mod download
	$(GO) mod tidy

tidy:
	$(GO) mod tidy

install-swag:
	$(GO) install github.com/swaggo/swag/cmd/swag@latest

docs:
	$(SWAG) init -g cmd/api/main.go -o docs

run: docs
	$(GO) run $(MAIN_PACKAGE)

dev: run

build: docs
	$(MKDIR_BIN)
	$(GO) build -o $(BINARY_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

ifeq ($(OS),Windows_NT)
check-env:
	powershell -NoProfile -ExecutionPolicy Bypass -Command '$$required = "GOOGLE_CLIENT_ID","GOOGLE_CLIENT_SECRET","GOOGLE_REDIRECT_URL","DATABASE_URL"; if (-not (Test-Path ".env")) { Write-Error ".env file not found"; exit 1 }; $$values = @{}; Get-Content ".env" | Where-Object { $$_.Trim() -and -not $$_.TrimStart().StartsWith("#") } | ForEach-Object { $$parts = $$_.Split("=", 2); if ($$parts.Count -eq 2) { $$values[$$parts[0].Trim()] = $$parts[1].Trim() } }; $$missing = @($$required | Where-Object { -not $$values.ContainsKey($$_) -or [string]::IsNullOrWhiteSpace($$values[$$_]) }); if ($$missing.Count -gt 0) { Write-Error ("Missing required environment variables: " + ($$missing -join ", ")); exit 1 }; Write-Host "Required environment variables are configured."'
else
check-env:
	@test -f .env || { echo ".env file not found"; exit 1; }
	@for key in GOOGLE_CLIENT_ID GOOGLE_CLIENT_SECRET GOOGLE_REDIRECT_URL DATABASE_URL; do \
		if ! grep -Eq "^[[:space:]]*$$key[[:space:]]*=[[:space:]]*[^[:space:]#]" .env; then \
			echo "Missing or empty environment variable: $$key"; exit 1; \
		fi; \
	done
	@echo "Required environment variables are configured."
endif

clean:
	$(CLEAN_BIN)
