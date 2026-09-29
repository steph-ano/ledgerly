.PHONY: test test-ledger test-bnpl build build-ledger build-bnpl docker-up docker-down

test: test-ledger test-bnpl

test-ledger:
	@echo "==> Running Ledger tests (unit, property, integration)..."
	cd services/ledger && go test -v -timeout 180s ./...

test-bnpl:
	@echo "==> Running BNPL tests (unit, property, integration, e2e)..."
	cd services/bnpl && go test -v -timeout 180s ./...

build: build-ledger build-bnpl

build-ledger:
	@echo "==> Compiling Ledger service..."
	cd services/ledger && go build -o bin/ledger ./cmd/api/main.go

build-bnpl:
	@echo "==> Compiling BNPL service (api and worker)..."
	cd services/bnpl && go build -o bin/bnpl-api ./cmd/api/main.go
	cd services/bnpl && go build -o bin/bnpl-worker ./cmd/worker/main.go

docker-up:
	@echo "==> Starting containers..."
	docker compose up -d --build

docker-down:
	@echo "==> Stopping containers..."
	docker compose down
