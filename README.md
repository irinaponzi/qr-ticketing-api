# Entradas QR - Ticket Sales & Validation System

A distributed system for selling event tickets with QR codes and validating them at the venue.

## Architecture

Three independent services communicating via RabbitMQ:

- **Ticket API** (`:8080`): Event creation and ticket purchasing. Publishes domain events.
- **QR Worker**: Asynchronous QR code generation and email delivery. Consumes `purchase.completed` events.
- **Validator API** (`:8081`): Validates QR codes at the venue using dual validation (Redis cache + live HTTP fallback). Publishes `ticket.used` events for bidirectional reconciliation. Includes IP-based rate limiting.

### Communication Pattern

```
                     ┌──────────────────┐
  POST /purchases ──>│   Ticket API     │──> ticket.created ──> Validator API
                     │   (:8080)        │──> purchase.completed ──> QR Worker
                     │                  │<── ticket.used ──── Validator API
                     └──────────────────┘
                                                                    │
                     ┌──────────────────┐                    ┌──────┴───────┐
  POST /validate ──> │  Validator API   │<── HTTP fallback ──│  QR Worker   │──> MailHog
  (rate limited)     │   (:8081)        │──> ticket.used ──> │  (consumer)  │
                     └──────────────────┘    Ticket API      └──────────────┘
```

- **Pub/Sub** (RabbitMQ): Syncs ticket lifecycle events to the validator Redis cache and triggers QR generation.
- **HTTP Fallback**: If a ticket is not found locally, the validator queries the ticket service directly.
- **Eventual Consistency** with **Live Fallback** for last-minute purchases.

### Design Patterns

- **DDD** with strict private attributes and factory methods
- **Ports & Adapters** (Hexagonal Architecture)
- **Repository Pattern** (interfaces in domain, implementations in storage)
- **Eventual Consistency** + Live Fallback
- **Idempotent consumers** (RabbitMQ)
- **Async workers** for I/O-heavy tasks (QR generation, email)
- **Bidirectional reconciliation** (ticket.used events flow back to Ticket API)
- **IP-based rate limiting** on validation endpoint (token bucket algorithm)

## Tech Stack

| Layer              | Technology                                          |
|--------------------|-----------------------------------------------------|
| **Language**       | Go 1.22+ with [chi](https://github.com/go-chi/chi) |
| **Database**       | MySQL 8.0 (tickets), Redis 7 (validator cache)      |
| **Message Broker** | RabbitMQ 3 (topic exchange)                         |
| **QR Generation**  | go-qrcode                                           |
| **Email**          | SMTP (MailHog for dev)                              |
| **Metrics**        | Prometheus + promhttp                               |
| **Logs**           | slog (JSON) → Loki                                  |
| **Dashboards**     | Grafana                                             |
| **Linting**        | golangci-lint v2                                    |
| **Testing**        | go test + sqlmock                                   |

## Quick Start

### 1. Start infrastructure

```bash
make infra
```

This starts:

| Service         | Port(s)      | Description                    |
|-----------------|--------------|--------------------------------|
| MySQL Tickets   | 3306         | Ticket service database        |
| Redis           | 6379         | Validator ticket cache         |
| RabbitMQ        | 5672 / 15672 | Message broker / Management UI |
| MailHog         | 1025 / 8025  | SMTP server / Web UI           |
| Prometheus      | 9090         | Metrics scraper                |
| Grafana         | 3000         | Dashboards (admin/admin)       |
| Loki            | 3100         | Log aggregation                |

### 2. Run services

```bash
# Terminal 1
make run-ticket

# Terminal 2
make run-validator

# Terminal 3
make run-qr-worker
```

### 3. Configure environment

```bash
cp .env.example .env
# Edit .env with your values — especially COGNITO_USER_POOL_ID
```

The `.env` file is gitignored. Never commit it. `.env.example` is the template committed to the repo.

### 4. Test the flow

All endpoints require a Cognito JWT. Use **boto3** to obtain one (curl does not work with new Cognito pools):

```python
# pip install boto3 --user
python3 << 'EOF'
import boto3
from botocore import UNSIGNED
from botocore.config import Config

client = boto3.client('cognito-idp', region_name='us-east-1', config=Config(signature_version=UNSIGNED))
admin = client.initiate_auth(AuthFlow='USER_PASSWORD_AUTH', ClientId='<CLIENT_ID>',
    AuthParameters={'USERNAME': 'admin@test.com', 'PASSWORD': '<ADMIN_PASSWORD>'})
user = client.initiate_auth(AuthFlow='USER_PASSWORD_AUTH', ClientId='<CLIENT_ID>',
    AuthParameters={'USERNAME': 'user@test.com', 'PASSWORD': '<USER_PASSWORD>'})
print('ADMIN_TOKEN:', admin['AuthenticationResult']['AccessToken'])
print('USER_TOKEN:', user['AuthenticationResult']['AccessToken'])
EOF
```

Copy the tokens and use them below. Admin endpoints need a user in the `admin` group; purchase endpoints need the `user` group.

```bash
ADMIN_TOKEN="<your-cognito-access-or-id-token>"
USER_TOKEN="<your-cognito-access-or-id-token>"

# Create an event (admin)
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"name":"Rock Concert","location":"Stadium","date":"2026-06-15T20:00:00Z","capacity":1000,"ticket_price":150.00}'

# Buy tickets (user)
curl -X POST http://localhost:8080/purchases \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -d '{"buyer_email":"user@example.com","event_id":1,"quantity":2}'

# Check email with QR codes at MailHog
open http://localhost:8025

# Validate a ticket — use the HMAC-signed token from the QR code in the email (admin)
curl -X POST http://localhost:8081/validate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"token":"<SIGNED_TOKEN_FROM_EMAIL_QR>"}'
```

Or import the Postman collection from `docs/EntradasQR.postman_collection.json` and set the `admin_token` / `user_token` collection variables.

### 5. Dashboards & Tools

| Tool     | URL                    | Credentials   |
|----------|------------------------|---------------|
| Grafana  | http://localhost:3000  | admin / admin |
| Prometheus | http://localhost:9090 | —            |
| RabbitMQ | http://localhost:15672  | guest / guest |
| MailHog  | http://localhost:8025   | —             |

### 6. Run tests & lint

```bash
make test           # Run all tests (73+)
make test-cover     # Run with coverage report
make lint           # Run golangci-lint
```

## Observability

### Metrics (Prometheus)

Both HTTP services expose a `/metrics` endpoint scraped by Prometheus every 15s.

| Metric                            | Type      | Labels                 |
|-----------------------------------|-----------|------------------------|
| `http_requests_total`             | Counter   | method, path, status   |
| `http_request_duration_seconds`   | Histogram | method, path           |
| `events_created_total`            | Counter   | —                      |
| `tickets_purchased_total`         | Counter   | —                      |
| `tickets_validated_total`         | Counter   | result (valid/invalid) |
| `rabbitmq_events_published_total` | Counter   | routing_key            |
| `rabbitmq_events_consumed_total`  | Counter   | queue, status          |

### Logs

Structured JSON logs via `slog` to stdout across all three services:
- Event creation, purchase completion, ticket validation
- QR generation and email delivery (QR Worker)
- RabbitMQ events published/consumed
- HTTP fallback activation
- Errors with full context

### Dashboard

A pre-built Grafana dashboard ("Entradas QR - Overview") is auto-provisioned with panels for request rates, latency, business metrics, validation results, and RabbitMQ throughput.

## Project Structure

```
cmd/
  ticket-api/           # Ticket API entry point
  validator-api/        # Validator API entry point
  qr-worker/            # QR Worker entry point
internal/
  ticket/               # Ticket bounded context (entities, service, interfaces, tests)
    handler/            # HTTP handlers + tests
    storage/            # MySQL repository implementations + sqlmock tests
    adapter/            # RabbitMQ publisher, QR generator, email sender, QR worker consumer + tests
  validator/            # Validator bounded context (entities, service, interfaces, tests)
    handler/            # HTTP handlers + tests
    storage/            # Redis repository implementation + tests
    adapter/            # RabbitMQ consumer & publisher, HTTP fallback client
  platform/             # Shared infrastructure
    config/             # Environment config (envconfig)
    database/           # MySQL connection helper
    rabbitmq/           # RabbitMQ connection & topology
    metrics/            # Prometheus metrics & HTTP middleware
    middleware/          # Rate limiting + JWT auth middleware
configs/                # Prometheus, Loki, Grafana configurations
docs/                   # OpenAPI specs + Postman collection
migrations/             # SQL schema files
```

## RabbitMQ Topology

| Exchange        | Type  | Queue                           | Routing Key          | Consumer      |
|-----------------|-------|---------------------------------|----------------------|---------------|
| `ticket.events` | topic | `validator.ticket.created`      | `ticket.created`     | Validator API |
| `ticket.events` | topic | `validator.ticket.cancelled`    | `ticket.cancelled`   | Validator API |
| `ticket.events` | topic | `qr-worker.purchase.completed`  | `purchase.completed` | QR Worker     |
| `ticket.events` | topic | `ticket.ticket.used`             | `ticket.used`        | Ticket API    |

## API Documentation

- **OpenAPI (Ticket API)**: `docs/openapi-ticket-api.yaml`
- **OpenAPI (Validator API)**: `docs/openapi-validator-api.yaml`
- **Postman Collection**: `docs/EntradasQR.postman_collection.json`

Paste the YAML files into [Swagger Editor](https://editor.swagger.io/) to explore interactively.

## Environment Variables

Copy `.env.example` to `.env` and fill in your values. The real `.env` is gitignored — never commit it.

### Shared (all services)

| Variable            | Default                              | Description |
|---------------------|--------------------------------------|-------------|
| COGNITO_REGION      | `us-east-1`                          | AWS region of the Cognito User Pool |
| COGNITO_USER_POOL_ID | _(required)_                        | Cognito User Pool ID (e.g. `us-east-1_AbCdEf`) |
| HMAC_SECRET         | `change-me-in-production`            | Secret for signing/verifying QR tokens |
| RABBITMQ_URL        | `amqp://guest:guest@localhost:5672/` | RabbitMQ connection string |

### Ticket API

| Variable     | Default    |
|--------------|------------|
| PORT         | `8080`     |
| DB_HOST      | `localhost` |
| DB_PORT      | `3306`     |
| DB_USER      | `root`     |
| DB_PASSWORD  | `root`     |
| DB_NAME      | `tickets_db` |

### Validator API

| Variable           | Default                |
|--------------------|------------------------|
| PORT               | `8081`                 |
| REDIS_HOST         | `localhost`            |
| REDIS_PORT         | `6379`                 |
| TICKET_SERVICE_URL | `http://localhost:8080` |

### QR Worker

| Variable      | Default                  |
|---------------|--------------------------|
| SMTP_HOST     | `localhost`              |
| SMTP_PORT     | `1025`                   |
| SMTP_FROM     | `tickets@entradasqr.local` |
| SMTP_USER     | _(empty — no auth for MailHog)_ |
| SMTP_PASSWORD | _(empty)_                |
| QR_SIZE       | `256`                    |

## Makefile Targets

| Target             | Description                          |
|--------------------|--------------------------------------|
| `make build`       | Compile all three binaries           |
| `make run-ticket`  | Run Ticket API                       |
| `make run-validator` | Run Validator API                  |
| `make run-qr-worker` | Run QR Worker                     |
| `make test`        | Run all tests with verbose output    |
| `make test-cover`  | Run tests with coverage report       |
| `make lint`        | Run golangci-lint                    |
| `make infra`       | Start Docker infrastructure          |
| `make infra-down`  | Stop Docker infrastructure           |
| `make tidy`        | Run `go mod tidy`                    |
