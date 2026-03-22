# ledger-core

A double-entry fintech ledger API built in Go — modelled after the internal architecture of systems like Paystack and MTN Mobile Money.

Every money movement is recorded as a pair of ledger entries (debit + credit) inside a single atomic transaction. The ledger always balances. No money is ever created or destroyed.

## Stack

- **Go** + **Gin** — HTTP layer
- **GORM** + **PostgreSQL** — persistence
- **Double-entry bookkeeping** — every debit has a matching credit
- **Idempotency middleware** — safe retries on all write operations via `Idempotency-Key` header

## Architecture

Each domain follows a clean three-layer pattern:

```
Handler (HTTP) → Service (business logic) → Repository (database)
```

Domains are fully decoupled. Cross-domain dependencies (e.g. transfer needing account balances) are resolved through interfaces, not direct imports — avoiding circular dependencies.

```
cmd/
└── server/
    └── main.go               # Entry point, dependency wiring

internal/
├── account/                  # Account management
│   ├── model.go
│   ├── repository.go
│   ├── service.go
│   └── handler.go
├── transfer/                 # Atomic transfers
│   ├── model.go
│   ├── repository.go
│   ├── service.go
│   └── handler.go
├── ledger/                   # Transaction ledger
│   ├── model.go
│   ├── repository.go
│   ├── service.go
│   └── handler.go
├── idempotency/              # Request deduplication
│   ├── model.go
│   └── repository.go
└── reconciliation/           # Ledger integrity checks
    ├── service.go
    └── handler.go

pkg/
├── database/                 # PostgreSQL connection
├── middleware/               # Idempotency middleware
└── response/                 # Standardized response envelope
```

## Features

### Accounts
Create accounts with a name, email, and currency (GHS, USD, EUR). Balances are stored in minor units (pesewas for GHS, cents for USD) to avoid floating-point precision issues — a standard practice in financial systems.

### Deposits & Withdrawals
Every deposit and withdrawal uses `SELECT FOR UPDATE` inside a transaction to prevent race conditions when concurrent requests hit the same account. Overdrafts are rejected with a `422`.

### Transfers
Transfers between accounts are fully atomic — both the debit and the credit happen in a single transaction. If either fails, both roll back and no money moves.

To prevent deadlocks under concurrent transfers between the same pair of accounts, locks are always acquired in a consistent order (smaller UUID first).

### Transaction Ledger
Every deposit, withdrawal, and transfer writes one or more `Entry` records to the ledger. Each entry captures the account ID, direction (debit/credit), amount, source operation, and the account balance at that moment — making it straightforward to reconstruct a full account statement.

### Idempotency
All `POST` endpoints support an optional `Idempotency-Key` header. If a request is retried with the same key, the original response is returned from cache without re-executing the handler. Keys are scoped to a single endpoint and expire after 24 hours.

### Reconciliation
The `/reconciliation` endpoint verifies the core double-entry invariant:

```
total_credits - total_debits == sum of all account balances
```

It also runs a per-account check, flagging any account where the stored balance doesn't match what the ledger entries compute — surfacing any balance inconsistencies immediately.

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| POST | `/api/v1/accounts` | Create account |
| GET | `/api/v1/accounts/:id` | Get account and balance |
| POST | `/api/v1/accounts/:id/deposit` | Deposit funds |
| POST | `/api/v1/accounts/:id/withdraw` | Withdraw funds |
| POST | `/api/v1/transfers` | Transfer between accounts |
| GET | `/api/v1/transfers/:id` | Get transfer by ID |
| GET | `/api/v1/accounts/:id/transfers` | Transfer history for an account |
| GET | `/api/v1/accounts/:id/ledger` | Paginated ledger entries |
| GET | `/api/v1/reconciliation` | Full ledger reconciliation report |

## Getting Started

### Prerequisites

- Go 1.21+
- PostgreSQL 14+

### Setup

```bash
# Clone the repo
git clone https://github.com/GordenArcher/ledger-core.git
cd ledger-core

# Copy env file and fill in your values
cp .env.example .env

# Install dependencies
go mod tidy

# Run the server (auto-migrates on startup)
go run cmd/server/main.go
```

### Environment Variables

```env
SERVER_PORT=8080

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=ledger_core
DB_SSLMODE=disable
```

## Usage Examples

### Create an account
```bash
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{"owner_name": "Gorden Archer", "email": "gorden@example.com", "currency": "GHS"}'
```

### Deposit funds
```bash
# Amount is in minor units — 5000 = GHS 50.00
curl -X POST http://localhost:8080/api/v1/accounts/<id>/deposit \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: dep-001" \
  -d '{"amount": 5000}'
```

### Transfer between accounts
```bash
curl -X POST http://localhost:8080/api/v1/transfers \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: txn-001" \
  -d '{
    "from_account_id": "<sender-id>",
    "to_account_id": "<receiver-id>",
    "amount": 1000,
    "reference": "school fees"
  }'
```

### Check reconciliation
```bash
curl http://localhost:8080/api/v1/reconciliation
```

## Response Format

All endpoints return a consistent envelope:

```json
{
  "status": "success",
  "message": "Deposit successful",
  "http_status": 200,
  "data": { ... },
  "code": "DEPOSIT_SUCCESS"
}
```

Error responses follow the same shape with `"status": "error"` and an `errors` field.
