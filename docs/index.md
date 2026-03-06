# EntradasQR

**A modern event ticketing platform built with Go, following Domain-Driven Design principles.**

EntradasQR is a microservices-based system for managing event tickets with QR code generation, email delivery, and real-time validation. It demonstrates clean architecture patterns including Ports & Adapters, eventual consistency via message queues, and comprehensive observability.

---

## Key Features

| Feature | Description |
|---|---|
| **Event Management** | Create events with capacity control and automatic sold-count tracking |
| **Ticket Purchase** | Buy multiple tickets with QR code generation and email delivery |
| **QR Validation** | Real-time ticket validation with local cache + live fallback |
| **Pub/Sub Sync** | RabbitMQ-based eventual consistency between services |
| **Observability** | Prometheus metrics, Loki logs, Grafana dashboards |
| **Idempotent Consumers** | Safe message replay without data corruption |

---

## System Overview

```mermaid
graph LR
    subgraph "Ticket Service :8080"
        TA[Ticket API]
        TS[Ticket Service]
        TR[(MySQL<br/>tickets_db)]
    end

    subgraph "Message Broker"
        RMQ{{RabbitMQ}}
    end

    subgraph "QR Worker"
        QW[QR Worker]
        QR[QR Generator]
        EM[Email Sender]
    end

    subgraph "Validator Service :8081"
        VA[Validator API]
        VS[Validator Service]
        VR[(Redis)]
    end

    subgraph "Observability"
        P[Prometheus]
        L[Loki]
        G[Grafana]
    end

    Client -->|HTTP| TA
    TA --> TS
    TS --> TR
    TS -->|Publish| RMQ
    RMQ -->|purchase.completed| QW
    QW --> QR
    QW --> EM
    EM -->|SMTP| MH[MailHog]
    RMQ -->|ticket.created/cancelled| VS
    VS --> VR
    Scanner -->|HTTP| VA
    VA --> VS
    VS -.->|Fallback| TA

    TA -->|/metrics| P
    VA -->|/metrics| P
    P --> G
    L --> G
```

---

## Quick Links

- [Architecture Overview](architecture/overview.md) — How the system is designed
- [Getting Started](development/getting-started.md) — Run the project locally in minutes
- [API Reference](api/ticket-api.md) — Full HTTP endpoint documentation
- [Testing Strategy](development/testing.md) — Unit tests, mocks, and coverage

---

## Tech Stack

| Layer | Technology |
|---|---|
| **Language** | Go 1.22+ |
| **HTTP Router** | chi |
| **Database** | MySQL 8.0 |
| **Message Broker** | RabbitMQ 3 |
| **QR Generation** | go-qrcode |
| **Email** | SMTP (MailHog for dev) |
| **Metrics** | Prometheus + promhttp |
| **Logs** | slog (JSON) → Loki |
| **Dashboards** | Grafana |
| **Linting** | golangci-lint v2 |
| **Testing** | go test + sqlmock |
