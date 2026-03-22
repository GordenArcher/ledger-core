# ledger-core

A double-entry fintech ledger API built in Go — atomic transfers, idempotency, and real-time reconciliation. Inspired by the internals of systems like Paystack and MTN Mobile Money.

## Stack

- **Go** + **Gin** — HTTP layer
- **GORM** + **PostgreSQL** — persistence
- **Double-entry bookkeeping** — every debit has a matching credit
- **Idempotency keys** — safe retries on all write operations

## Features

- Account management (create, balance inquiry)
- Deposits and withdrawals
- Atomic transfers between accounts (no money created or lost)
- Full transaction ledger with history
- Idempotency on all write operations
- Reconciliation endpoint to verify ledger integrity

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

# Run the server
go run cmd/server/main.go
```

### API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /health | Health check |
| POST | /api/v1/accounts | Create account |
| GET | /api/v1/accounts/:id | Get account + balance |
| POST | /api/v1/accounts/:id/deposit | Deposit funds |
| POST | /api/v1/accounts/:id/withdraw | Withdraw funds |
| POST | /api/v1/transfers | Transfer between accounts |
| GET | /api/v1/accounts/:id/transactions | Transaction history |
| GET | /api/v1/reconciliation | Ledger reconciliation report |

## Architecture

Each domain (account, transaction, transfer, reconciliation) follows a clean three-layer pattern:

```
Handler (HTTP) → Service (business logic) → Repository (database)
```

This maps closely to Django's views → logic → ORM pattern.
