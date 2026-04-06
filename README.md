# Fintech Ledger System

A production-grade double-entry bookkeeping ledger built in Go. Designed for accuracy, idempotency, and reliability — every cent is accounted for.

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Prerequisites](#prerequisites)
- [Local Development](#local-development)
- [Running with Docker](#running-with-docker)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Testing](#testing)
- [Database Migrations](#database-migrations)
- [Observability](#observability)
- [Design Decisions](#design-decisions)

---

## Overview

This service implements a **double-entry bookkeeping ledger** — the accounting model used by every bank and financial institution. Every transaction must have equal debits and credits (`sum(debits) == sum(credits)`), making the ledger self-auditing by construction.

**Key capabilities:**

- Multi-currency account management (one account per user per currency)
- Atomic transfers with double-entry entries
- Idempotent transaction submission via `Idempotency-Key` header
- Real-time balance computation from committed entries
- Reliable event publishing via the **Transactional Outbox Pattern**
- Dead-letter queue for failed event processing
- Prometheus metrics on every HTTP endpoint

**Tech stack:**

| Concern | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP framework | [Gin](https://github.com/gin-gonic/gin) |
| Database | PostgreSQL 16 (pgx/v5) |
| Cache / Queue | Redis 7 (go-redis/v9) |
| Metrics | Prometheus + Grafana |
| Container | Docker (multi-stage, `scratch` base) |

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│  HTTP API  (cmd/api)          Worker  (cmd/worker)       │
│  Gin router + middleware       Outbox relay              │
│  Idempotency · Metrics         Consumer · DLQ producer   │
├──────────────────────────────────────────────────────────┤
│               Service Layer                              │
│   AccountService · TransferService                       │
├──────────────────────────────────────────────────────────┤
│               Repository Layer                           │
│   AccountRepo · TransactionRepo · OutboxRepo             │
├──────────────────────────────────────────────────────────┤
│               Domain Layer                               │
│   Account · Money · Transaction · Entry                  │
├───────────────────────┬──────────────────────────────────┤
│   PostgreSQL          │   Redis                          │
│   accounts            │   Idempotency cache              │
│   transactions        │   Event stream (XADD/XREAD)      │
│   entries             │   Dead-letter stream             │
│   outbox              │                                  │
└───────────────────────┴──────────────────────────────────┘
```

### Event flow (Outbox Pattern)

```
Transfer API call
     │
     ▼
DB Transaction ──► INSERT transaction + entries + outbox row  (atomic)
     │
     ▼
OutboxRelay (worker) polls DB every 5s
     │
     ▼
XADD ──► Redis Stream  ledger:events
     │
     ▼
Consumer group reads and processes events
     │
     ▼  (on repeated failure)
DLQProducer ──► Redis Stream  ledger:events:dlq
```

---

## Project Structure

```
.
├── cmd/
│   ├── api/main.go          # API server entry point
│   └── worker/main.go       # Background worker entry point
├── config/
│   └── config.go            # Environment variable loading
├── internal/
│   ├── domain/              # Core business logic (no external deps)
│   │   ├── account.go
│   │   ├── money.go         # Integer-arithmetic monetary values
│   │   └── transaction.go   # Double-entry transaction + entries
│   ├── handler/             # HTTP request/response layer
│   │   ├── handler.go
│   │   └── router.go
│   ├── middleware/
│   │   ├── idempotency.go   # Redis-backed request deduplication
│   │   └── metrics.go       # Prometheus instrumentation
│   ├── repository/          # PostgreSQL data access
│   │   ├── account_repo.go
│   │   ├── transaction_repo.go
│   │   └── outbox_repo.go
│   ├── service/             # Orchestration and business rules
│   │   ├── account_service.go
│   │   └── transfer_service.go
│   ├── worker/              # Background processing
│   │   ├── outbox_relay.go  # DB → Redis stream
│   │   ├── consumer.go      # Redis stream consumer group
│   │   └── dlq_producer.go  # Failed message → DLQ
│   └── testhelper/
│       └── db.go            # Shared test infrastructure
├── migrations/
│   ├── 001_accounts.sql
│   ├── 002_transactions.sql
│   └── 003_outbox.sql
├── deployments/
│   └── prometheus/
│       └── prometheus.yml
├── docker/
│   └── Dockerfile           # Multi-stage, scratch final image
├── docker-compose.yml
├── Makefile
└── .env.example
```

---

## Prerequisites

**Local development:**

| Tool | Version |
|---|---|
| Go | ≥ 1.25 |
| PostgreSQL | ≥ 16 |
| Redis | ≥ 7 |

**Docker deployment:**

| Tool | Version |
|---|---|
| Docker | ≥ 24 |
| Docker Compose | ≥ 2.20 |

---

## Local Development

### 1. Clone and set up environment

```bash
git clone <repo-url>
cd ledger-system

make env          # copies .env.example → .env
```

Edit `.env` if your local Postgres/Redis use non-default credentials.

### 2. Apply database migrations

```bash
make migrate
# or manually:
# psql "postgres://ledger:ledger@localhost:5432/ledger" -f migrations/001_accounts.sql
# psql "postgres://ledger:ledger@localhost:5432/ledger" -f migrations/002_transactions.sql
# psql "postgres://ledger:ledger@localhost:5432/ledger" -f migrations/003_outbox.sql
```

### 3. Run the API server

```bash
make run-api
# → api: listening on :8080
```

### 4. Run the background worker (separate terminal)

```bash
make run-worker
# → worker: running
```

### 5. Verify

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

---

## Running with Docker

All services — Postgres, Redis, API, Worker, Prometheus, Grafana — start with a single command.

```bash
make env        # create .env from .env.example (first time only)
make up         # docker compose up -d --build
```

| Service | URL |
|---|---|
| API | http://localhost:8080 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 (admin / admin) |

```bash
make logs       # follow all logs
make ps         # show container status
make down       # stop everything
```

Rebuild a single service without restarting everything:

```bash
make restart-api
make restart-worker
```

---

## Configuration

All configuration is read from environment variables (`.env` in development, injected secrets in production).

| Variable | Default | Description |
|---|---|---|
| `HTTP_PORT` | `8080` | Port the API server binds to |
| `POSTGRES_DSN` | `postgres://ledger:ledger@localhost:5432/ledger?sslmode=disable` | PostgreSQL connection string |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASS` | _(empty)_ | Redis password |
| `REDIS_DB` | `0` | Redis database index |

In production, set `GIN_MODE=release` to disable debug output.

---

## API Reference

### Accounts

#### Create account

```
POST /v1/accounts
Content-Type: application/json

{
  "user_id":  "550e8400-e29b-41d4-a716-446655440000",
  "currency": "USD"
}
```

Returns `201 Created`:

```json
{
  "id":         "069e67a2-9b46-47c0-a530-aef183d8e343",
  "user_id":    "550e8400-e29b-41d4-a716-446655440000",
  "currency":   "USD",
  "created_at": "2026-04-06T09:19:48.594137Z"
}
```

Errors: `400` invalid currency, `409` account already exists for this currency.

---

#### Get account

```
GET /v1/accounts/:id
```

Returns `200 OK` or `404 Not Found`.

---

#### Get account balance

```
GET /v1/accounts/:id/balance
```

Returns `200 OK`:

```json
{
  "account_id": "069e67a2-9b46-47c0-a530-aef183d8e343",
  "currency":   "USD",
  "balance":    10000
}
```

> Amounts are in **minor units** (e.g. cents for USD). `10000` = $100.00.

---

#### List accounts for a user

```
GET /v1/users/:user_id/accounts
```

Returns `200 OK` with an array of account objects.

---

### Transfers

#### Initiate a transfer

```
POST /v1/transfers
Content-Type: application/json
Idempotency-Key: <unique-key>

{
  "reference_id":    "pay-invoice-8821",
  "from_account_id": "069e67a2-9b46-47c0-a530-aef183d8e343",
  "to_account_id":   "9cbcab11-3bb2-49e9-bbaa-8f3665161be2",
  "amount":          5000,
  "currency":        "USD"
}
```

Returns `201 Created`:

```json
{
  "id":           "a3f1b2c4-...",
  "reference_id": "pay-invoice-8821",
  "status":       "committed",
  "created_at":   "2026-04-06T09:30:00Z",
  "entries": [
    { "id": "...", "account_id": "069e...", "amount": 5000, "currency": "USD", "type": "debit",  ... },
    { "id": "...", "account_id": "9cbc...", "amount": 5000, "currency": "USD", "type": "credit", ... }
  ]
}
```

Errors: `400` bad request, `404` account not found, `409` duplicate reference, `422` insufficient funds.

**Idempotency:** If the same `Idempotency-Key` header is sent again within 24 hours, the original response is returned from Redis without re-processing. This is the mechanism for safe retries.

---

#### Get transaction by ID

```
GET /v1/transactions/:id
```

#### Get transaction by reference

```
GET /v1/transactions/reference/:ref_id
```

Both return `200 OK` with the full transaction + entries, or `404 Not Found`.

---

### Health and metrics

```
GET /healthz          # liveness probe
GET /metrics          # Prometheus metrics
```

---

## Testing

The test suite covers domain logic (unit), repositories (integration against real Postgres), services, and HTTP handlers end-to-end.

```bash
make test
# go test -race -cover -p 1 ./...
```

> `-p 1` runs packages sequentially. Integration tests truncate tables between cases, so parallel package execution would cause race conditions across packages sharing one database.

### Run a specific package

```bash
go test -v -race ./internal/domain/...
go test -v -race -p 1 ./internal/repository/...
go test -v -race -p 1 ./internal/service/...
go test -v -race -p 1 ./internal/handler/...
```

### Test coverage

```bash
go test -p 1 -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### What is tested

| Layer | Tests |
|---|---|
| `domain/Money` | Arithmetic, currency mismatch, negative amounts |
| `domain/Transaction` | Invariant enforcement, state transitions, entry filtering |
| `repository/Account` | Create, get, list, duplicate detection, balance query |
| `repository/Transaction` | Atomic create (tx + entries + outbox), get by ID and reference, duplicate reference |
| `service/Transfer` | Happy path with balance verification, insufficient funds, currency mismatch, idempotency |
| `handler` | All HTTP endpoints, correct status codes, idempotency middleware, Prometheus metrics |

---

## Database Migrations

Migrations are plain SQL files applied in order. There is no migration framework — run them once against a fresh database.

```bash
make migrate DSN="postgres://user:pass@host:5432/dbname?sslmode=require"
```

| File | Creates |
|---|---|
| `001_accounts.sql` | `accounts` table, unique index on `(user_id, currency)` |
| `002_transactions.sql` | `transactions`, `entries` tables with `transaction_status` and `entry_type` enums |
| `003_outbox.sql` | `outbox` table for the transactional outbox pattern |

**Schema invariants enforced at the database level:**

- `accounts`: `currency` must match `^[A-Z]{3}$`
- `entries`: `amount` must be `> 0`
- `entries`: foreign key to `transactions` and `accounts`
- `transactions.reference_id`: unique — duplicate transactions are rejected

---

## Observability

### Prometheus metrics

Available at `GET /metrics`. Emitted per request:

| Metric | Type | Labels |
|---|---|---|
| `http_requests_total` | Counter | `method`, `path`, `status` |
| `http_request_duration_seconds` | Histogram | `method`, `path` |

### Grafana

Default dashboard available at http://localhost:3000 (admin / admin) after `make up`. Add a Prometheus data source pointing to `http://prometheus:9090`, then build panels from the metrics above.

### Logs

Structured log lines are written to stdout by Gin and the worker. Forward them to your log aggregator (Loki, CloudWatch, Datadog, etc.) via the container runtime's logging driver.

### Health check

```
GET /healthz → 200 {"status":"ok"}
```

Use this as the liveness probe in Kubernetes or the healthcheck in Docker Compose.

---

## Design Decisions

### Double-entry bookkeeping

Every monetary movement creates two entries: a debit and a credit. The invariant `sum(debits) == sum(credits)` is enforced both in the domain layer (`Transaction.validate()`) and implicitly by the fact that balance is always computed from entries — there is no mutable balance field that can drift.

### Money as integer minor units

`Money` stores amounts as `int64` minor units (cents for USD, pence for GBP, etc.). Floating-point is never used for monetary arithmetic. This eliminates rounding errors by design.

### Idempotency

Transfers are safe to retry. The `Idempotency-Key` header caches the full HTTP response in Redis for 24 hours. The `reference_id` field enforces uniqueness at the database level as a second layer of protection.

### Transactional Outbox Pattern

Rather than publishing events directly from the API (which can fail after the DB commit, producing lost events), every committed transaction writes an outbox row in the **same database transaction**. The `OutboxRelay` worker polls for unprocessed rows and publishes them to a Redis stream, then marks them processed. This guarantees at-least-once delivery without distributed transactions.

### FOR UPDATE SKIP LOCKED

The outbox query uses `FOR UPDATE SKIP LOCKED`, which allows multiple worker replicas to poll the outbox concurrently without conflicts or double-processing.

### Scratch Docker image

The final image is built `FROM scratch` — no shell, no OS utilities, no package manager. The attack surface is the application binary alone. CA certificates and timezone data are copied explicitly from the builder stage.
