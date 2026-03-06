# Architecture Overview

EntradasQR follows a **microservices architecture** with two bounded contexts and three services communicating through asynchronous events via RabbitMQ.

---

## High-Level Architecture

```mermaid
C4Context
    title EntradasQR - System Context

    Person(buyer, "Buyer", "Purchases event tickets")
    Person(scanner, "Scanner", "Validates tickets at venue")

    System(ticketSvc, "Ticket Service", "Manages events, purchases, and ticket lifecycle")
    System(qrWorker, "QR Worker", "Generates QR codes and sends confirmation emails")
    System(validatorSvc, "Validator Service", "Validates tickets at entry points")

    SystemQueue(rmq, "RabbitMQ", "Async event bus for ticket lifecycle events")
    SystemDb(ticketsDb, "Tickets DB", "MySQL - Source of truth for tickets")
    SystemDb(redisDb, "Redis", "Key-value store - Fast ticket lookups")
    System_Ext(mailhog, "MailHog", "SMTP server for dev email")

    Rel(buyer, ticketSvc, "Creates events, buys tickets", "HTTP")
    Rel(scanner, validatorSvc, "Validates ticket QR codes", "HTTP")
    Rel(ticketSvc, rmq, "Publishes ticket.created / ticket.cancelled / purchase.completed")
    Rel(validatorSvc, rmq, "Publishes ticket.used")
    Rel(rmq, validatorSvc, "Delivers ticket events")
    Rel(rmq, qrWorker, "Delivers purchase.completed")
    Rel(rmq, ticketSvc, "Delivers ticket.used")
    Rel(qrWorker, mailhog, "Sends ticket emails", "SMTP")
    Rel(ticketSvc, ticketsDb, "Read/Write")
    Rel(validatorSvc, redisDb, "Read/Write")
    Rel(validatorSvc, ticketSvc, "Live fallback", "HTTP")
```

---

## Design Principles

### 1. Domain-Driven Design (DDD)
Each service encapsulates its own bounded context with rich domain entities, value objects, and repository interfaces. Business rules live **exclusively** in the domain layer.

### 2. Ports & Adapters (Hexagonal Architecture)
The domain defines **ports** (interfaces) for external dependencies. Concrete implementations (**adapters**) are injected at startup, keeping the domain free of infrastructure concerns.

### 3. Eventual Consistency
The Validator Service maintains a **local copy** of ticket data, synced asynchronously via RabbitMQ. This allows sub-millisecond validation at venue entry points without depending on the Ticket Service being available.

### 4. Live Fallback
When a ticket is not found in the local cache (e.g., race condition during sync), the Validator falls back to a **synchronous HTTP call** to the Ticket Service with a 3-second timeout.

### 5. Idempotent Consumers
RabbitMQ consumers are designed to handle **message replays** safely. Creating an already-existing ticket or cancelling an already-cancelled ticket are no-ops.

### 6. Bidirectional Reconciliation
When the Validator Service marks a ticket as used, it publishes a `ticket.used` event back to RabbitMQ. The Ticket Service consumes this event and updates the ticket status in its own database, keeping both services in sync.

### 7. Rate Limiting
The Validator API uses **IP-based rate limiting** (token bucket algorithm) to protect against brute-force UUID guessing attacks on the validation endpoint.

---

## Service Boundaries

```mermaid
graph TB
    subgraph "Ticket Bounded Context"
        E[Event]
        T[Ticket]
        P[Purchase]
        E -->|"has many"| T
        P -->|"has many"| T
    end

    subgraph "Validator Bounded Context"
        VT[ValidTicket]
    end

    T -.->|"synced via RabbitMQ"| VT
```

| Aspect | Ticket Service | QR Worker | Validator Service |
|---|---|---|---|
| **Responsibility** | Event CRUD, ticket purchase, cancellation | QR generation, email delivery | Ticket validation at venue |
| **Database** | `tickets_db` MySQL (port 3306) | None | Redis (port 6379) |
| **HTTP Port** | 8080 | — (consumer only) | 8081 |
| **Entities** | Event, Ticket, Purchase | — (uses domain events) | ValidTicket |
| **Role** | Source of truth | Async processor | Read-optimized replica |

---

## Data Flow

### Purchase Flow

```mermaid
sequenceDiagram
    participant B as Buyer
    participant TA as Ticket API
    participant TS as TicketService
    participant DB as tickets_db
    participant RMQ as RabbitMQ
    participant QW as QR Worker
    participant MH as MailHog
    participant VS as ValidatorService
    participant RD as Redis

    B->>TA: POST /purchases
    TA->>TS: Purchase(input)
    TS->>DB: Get Event
    TS->>DB: Reserve Tickets (sold_count += qty)
    TS->>DB: Add Purchase
    loop For each ticket
        TS->>DB: Add Ticket
        TS->>RMQ: Publish ticket.created
    end
    TS->>RMQ: Publish purchase.completed
    TS-->>TA: PurchaseResult
    TA-->>B: 201 Created

    RMQ-->>VS: Deliver ticket.created
    VS->>RD: SET ticket:{code} (idempotent)

    RMQ-->>QW: Deliver purchase.completed
    QW->>QW: Sign code (HMAC) + Generate QR
    QW->>MH: Send email with QR attachments
```

### Validation Flow

```mermaid
sequenceDiagram
    participant S as Scanner
    participant VA as Validator API
    participant VS as ValidatorService
    participant RD as Redis
    participant TA as Ticket API

    S->>VA: POST /validate {ticket_code: "code.sig"}
    VA->>VA: Rate limit check (IP)
    VA->>VA: Verify HMAC signature
    VA->>VS: ValidateTicket(code)
    VS->>RD: GET ticket:{code}

    alt Found in Redis
        VS->>VS: Check status (active/used/cancelled)
        alt Active
            VS->>RD: SET ticket:{code} (used)
            VS->>RMQ: Publish ticket.used
            VS-->>VA: Valid ✅
        else Used/Cancelled
            VS-->>VA: Invalid ❌
        end
    else Not found in Redis
        VS->>TA: POST /tickets/lookup (fallback)
        alt Found & active
            VS->>RD: SET ticket:{code} (sync)
            VS->>RMQ: Publish ticket.used
            VS-->>VA: Valid ✅
        else Not found
            VS-->>VA: Invalid ❌
        end
    end

    VA-->>S: JSON response

    RMQ-->>TA: Deliver ticket.used
    TA->>DB: MarkAsUsed (idempotent)
```
