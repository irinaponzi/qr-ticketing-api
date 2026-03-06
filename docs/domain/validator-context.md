# Validator Bounded Context

The Validator context maintains a **read-optimized local copy** of ticket data in Redis for fast validation at venue entry points. It verifies HMAC-signed QR tokens, receives updates asynchronously via RabbitMQ, and falls back to the Ticket Service when data is not yet synced.

---

## Entity: ValidTicket

A `ValidTicket` is a projection of a ticket optimized for the validation use case.

| Field | Type | Description |
|---|---|---|
| `id` | `int` | Unique identifier (local) |
| `code` | `string` | UUID code matching the original ticket |
| `eventID` | `int` | Associated event |
| `status` | `ValidTicketStatus` | Current lifecycle state |
| `usedAt` | `*time.Time` | When the ticket was validated |
| `syncedAt` | `time.Time` | When the ticket was synced from RabbitMQ |
| `updatedAt` | `time.Time` | Last update timestamp |

**State Machine:**

```mermaid
stateDiagram-v2
    [*] --> active : NewValidTicket() / SyncTicketCreated
    active --> used : MarkAsUsed() / ValidateTicket
    active --> cancelled : MarkAsCancelled() / SyncTicketCancelled
    used --> [*]
    cancelled --> [*]
```

| Status | Description |
|---|---|
| `active` | Ticket is valid and can be used at the venue |
| `used` | Ticket has been scanned and validated |
| `cancelled` | Ticket has been revoked by the Ticket Service |

---

## Domain Service: ValidatorService

The `ValidatorService` handles ticket validation and event synchronization.

### Dependencies (Ports)

```mermaid
graph LR
    VS[ValidatorService]
    VS --> VTR[ValidTicketRepository]
    VS --> TSC[TicketServiceClient]
    VH[ValidatorHandler] --> TS[TokenSigner]
    VH --> VS
```

### ValidateTicket Flow

```mermaid
flowchart TD
    A[Receive signed token] --> HMAC{Verify HMAC signature}
    HMAC -->|invalid| FAIL[Return Invalid: tampered token ❌]
    HMAC -->|valid| B{Found in Redis?}
    B -->|Yes| C{Status?}
    C -->|active| D[MarkAsUsed + Update Redis]
    D --> E[Return Valid ✅]
    C -->|used| F[Return Invalid: already used ❌]
    C -->|cancelled| G[Return Invalid: cancelled ❌]
    B -->|No| H[Fallback: POST /tickets/lookup]
    H --> I{Found?}
    I -->|Yes & active| J[Sync to Redis + Return Valid ✅]
    I -->|Yes & not active| K[Return Invalid ❌]
    I -->|No| L[Return Invalid: not found ❌]
    H -->|Error| M[Return error]
```

!!! info "Fallback Timeout"
    The HTTP fallback uses a **3-second timeout** to avoid blocking the validation flow. If the Ticket Service is down, the scanner will receive an error rather than hanging indefinitely.

### SyncTicketCreated

1. Check if ticket already exists by code (idempotency)
2. If exists → no-op (message replay safe)
3. If not exists → create `ValidTicket` with `active` status

### SyncTicketCancelled

1. Load ticket by code
2. If not found → no-op (out-of-order message)
3. If found → call `MarkAsCancelled()` and persist

---

## Consistency Model

```mermaid
sequenceDiagram
    participant TS as Ticket Service
    participant RMQ as RabbitMQ
    participant VS as Validator Service
    participant RD as Redis

    Note over TS,RD: Eventual Consistency Window

    TS->>RMQ: Publish ticket.created
    Note right of RMQ: ~10-100ms latency
    RMQ->>VS: Deliver message
    VS->>RD: SET ticket:{code}

    Note over TS,RD: During the consistency window,<br/>fallback to Ticket Service via HTTP
```

| Scenario | Behavior |
|---|---|
| Normal sync | Ticket available in Redis within ~100ms |
| Sync delay | Fallback HTTP call to Ticket Service |
| Ticket Service down | Error returned to scanner |
| Duplicate message | Idempotent: no duplicate records |
| Out-of-order cancel | No-op if ticket not yet synced |
