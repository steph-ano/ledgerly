.PHONY: test test-unit test-property test-integration build run docker-up docker-down

test: test-unit test-property test-integration

test-unit:
	@echo "==> Running domain unit tests..."
	cd services/ledger && go test -v ./internal/domain/...

test-property:
	@echo "==> Running property-based tests..."
	cd services/ledger && go test -v -run "TestProperty_" ./internal/domain/...

test-integration:
	@echo "==> Running PostgreSQL and HTTP integration tests..."
	cd services/ledger && go test -v -timeout 180s ./internal/storage/postgres/... ./internal/api/http/...

build:
	@echo "==> Compiling ledger service..."
	cd services/ledger && go build -o bin/ledger ./cmd/api/main.go

run:
	@echo "==> Running ledger service locally..."
	cd services/ledger && go run ./cmd/api/main.go

docker-up:
	@echo "==> Starting containers..."
	docker compose up -d --build

docker-down:
	@echo "==> Stopping containers..."
	docker compose down
