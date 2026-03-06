# Getting Started

Step-by-step guide to set up and run the EntradasQR project locally.

---

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| **Go** | 1.21+ | Application runtime |
| **Docker** + **Docker Compose** | Latest | Infrastructure services |
| **Make** | Any | Build automation |
| **golangci-lint** | v2+ | Code linting (auto-installed by `make lint`) |

---

## Quick Start

### 1. Clone the repository

```bash
git clone https://github.com/iponzi/entradasQR.git
cd entradasQR
```

### 2. Start infrastructure

```bash
make infra
```

This launches MySQL, Redis, RabbitMQ, MailHog, Prometheus, Loki, and Grafana.

Wait ~10 seconds for MySQL to initialize the schemas.

### 3. Build all services

```bash
make build
```

Compiles three binaries into `bin/`:

- `bin/ticket-api`
- `bin/validator-api`
- `bin/qr-worker`

### 4. Run the services

Open three terminals:

=== "Terminal 1 — Ticket API"

    ```bash
    make run-ticket
    ```

=== "Terminal 2 — Validator API"

    ```bash
    make run-validator
    ```

=== "Terminal 3 — QR Worker"

    ```bash
    make run-qr-worker
    ```

### 5. Test the flow

```bash
# Create an event
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"name":"Rock Festival","location":"Luna Park","date":"2026-07-15T20:00:00Z","capacity":1000,"ticket_price":150.00}'

# Purchase tickets (triggers QR generation + email via worker)
curl -X POST http://localhost:8080/purchases \
  -H "Content-Type: application/json" \
  -d '{"buyer_email":"fan@example.com","event_id":1,"quantity":2}'

# Check email at MailHog (QR codes contain HMAC-signed tokens)
open http://localhost:8025

# Validate a ticket (use the HMAC-signed token from the QR code)
curl -X POST http://localhost:8081/validate \
  -H "Content-Type: application/json" \
  -d '{"ticket_code":"<signed-token-from-qr>"}'
```

---

## Environment Variables

### Ticket API

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DB_HOST` | `localhost` | MySQL host |
| `DB_PORT` | `3306` | MySQL port |
| `DB_USER` | `root` | MySQL user |
| `DB_PASSWORD` | `root` | MySQL password |
| `DB_NAME` | `tickets_db` | Database name |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | RabbitMQ connection |

### Validator API

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8081` | HTTP listen port |
| `REDIS_HOST` | `localhost` | Redis server hostname |
| `REDIS_PORT` | `6379` | Redis server port |
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | RabbitMQ connection |
| `TICKET_SERVICE_URL` | `http://localhost:8080` | Fallback URL |
| `HMAC_SECRET` | `change-me-in-production` | HMAC-SHA256 secret for token verification |

### QR Worker

| Variable | Default | Description |
|---|---|---|
| `RABBITMQ_URL` | `amqp://guest:guest@localhost:5672/` | RabbitMQ connection |
| `SMTP_HOST` | `localhost` | SMTP server host |
| `SMTP_PORT` | `1025` | SMTP server port |
| `SMTP_FROM` | `tickets@entradasqr.local` | Sender email address |
| `SMTP_USER` | _(empty)_ | SMTP username (empty = no auth, e.g. MailHog) |
| `SMTP_PASSWORD` | _(empty)_ | SMTP password (empty = no auth) |
| `QR_SIZE` | `256` | QR code image size in pixels |
| `HMAC_SECRET` | `change-me-in-production` | HMAC-SHA256 secret for token signing |

---

## Makefile Targets

| Target | Description |
|---|---|
| `make build` | Compile all three binaries |
| `make run-ticket` | Run Ticket API |
| `make run-validator` | Run Validator API |
| `make run-qr-worker` | Run QR Worker |
| `make test` | Run all tests with verbose output |
| `make test-cover` | Run tests with coverage report |
| `make lint` | Run golangci-lint (auto-installs if missing) |
| `make infra` | Start Docker infrastructure |
| `make infra-down` | Stop Docker infrastructure |
| `make tidy` | Run `go mod tidy` |

---

## Project Structure

```
entradasQR/
├── cmd/
│   ├── ticket-api/          # Ticket API entry point
│   ├── validator-api/       # Validator API entry point
│   └── qr-worker/           # QR Worker entry point
├── internal/
│   ├── ticket/              # Ticket bounded context
│   │   ├── adapter/         # RabbitMQ publisher, QR generator, email sender
│   │   ├── handler/         # HTTP handlers
│   │   └── storage/         # MySQL repositories
│   ├── validator/           # Validator bounded context
│   │   ├── adapter/         # RabbitMQ consumer, HTTP client
│   │   ├── handler/         # HTTP handlers
│   │   └── storage/         # Redis repository
│   └── platform/            # Shared infrastructure
│       ├── config/          # Environment configuration
│       ├── database/        # MySQL connection
│       ├── metrics/         # Prometheus metrics & middleware
│       └── rabbitmq/        # RabbitMQ connection & topology
├── migrations/              # SQL schema files
├── configs/                 # Prometheus, Loki, Grafana configs
├── docs/                    # MkDocs documentation
├── docker-compose.yml       # Infrastructure services
├── Makefile                 # Build automation
└── mkdocs.yml               # Documentation site config
```
