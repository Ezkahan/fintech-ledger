.PHONY: build run-api run-worker test lint fmt tidy vet migrate up down logs ps restart-api restart-worker env

GO      := go
GOFLAGS :=
DSN     ?= postgres://ledger:ledger@localhost:5432/ledger?sslmode=disable

build:
	@mkdir -p bin
	$(GO) build $(GOFLAGS) -o bin/api    ./cmd/api
	$(GO) build $(GOFLAGS) -o bin/worker ./cmd/worker

run-api:
	@source .env 2>/dev/null; $(GO) run ./cmd/api

run-worker:
	@source .env 2>/dev/null; $(GO) run ./cmd/worker

test:
	$(GO) test -race -cover -p 1 ./...

lint:
	golangci-lint run ./...

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

vet:
	$(GO) vet ./...

migrate:
	@for f in migrations/*.sql; do \
		echo "Applying $$f..."; \
		psql "$(DSN)" -f "$$f"; \
	done

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

ps:
	docker compose ps

restart-api:
	docker compose up -d --build api

restart-worker:
	docker compose up -d --build worker

env:
	@test -f .env || (cp .env.example .env && echo "Created .env from .env.example")
