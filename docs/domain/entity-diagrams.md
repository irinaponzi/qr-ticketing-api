# Entity Diagrams

Complete class diagrams for all domain entities across both bounded contexts.

---

## Ticket Context — Full Class Diagram

```mermaid
classDiagram
    class Event {
        -id int
        -attributes eventAttributes
        +NewEvent(id, name, location, date, capacity, ticketPrice) Event, error
        +NewEventFromRepository(...) Event
        +ID() int
        +Name() string
        +Location() string
        +Date() time.Time
        +Capacity() int
        +TicketPrice() float64
        +SoldCount() int
        +CreatedAt() time.Time
        +UpdatedAt() time.Time
        +AvailableTickets() int
        +HasAvailableTickets() bool
        +ReserveTickets(quantity int) error
        +UpdateName(name string) error
        +SetID(id int)
    }

    class Ticket {
        -id int
        -attributes ticketAttributes
        +NewTicket(id, eventID, purchaseID) Ticket, error
        +NewTicketFromRepository(...) Ticket
        +ID() int
        +Code() string
        +EventID() int
        +PurchaseID() int
        +Status() TicketStatus
        +UsedAt() *time.Time
        +CreatedAt() time.Time
        +UpdatedAt() time.Time
        +IsValid() bool
        +MarkAsUsed() error
        +Cancel() error
    }

    class Purchase {
        -id int
        -attributes purchaseAttributes
        +NewPurchase(id, buyerEmail, eventID, qty, price) Purchase, error
        +NewPurchaseFromRepository(...) Purchase
        +ID() int
        +BuyerEmail() string
        +EventID() int
        +Quantity() int
        +TotalPrice() float64
        +Tickets() []*Ticket
        +CreatedAt() time.Time
        +AddTicket(ticket *Ticket) error
        +TicketCodes() []string
    }

    class TicketStatus {
        <<enumeration>>
        emitted
        used
        cancelled
    }

    Event "1" --> "*" Ticket : contains
    Purchase "1" --> "*" Ticket : groups
    Ticket --> TicketStatus : has
```

---

## Validator Context — Full Class Diagram

```mermaid
classDiagram
    class ValidTicket {
        -id int
        -attributes validTicketAttributes
        +NewValidTicket(id, code, eventID) ValidTicket, error
        +NewValidTicketFromRepository(...) ValidTicket
        +ID() int
        +Code() string
        +EventID() int
        +Status() ValidTicketStatus
        +UsedAt() *time.Time
        +SyncedAt() time.Time
        +UpdatedAt() time.Time
        +IsActive() bool
        +MarkAsUsed() error
        +MarkAsCancelled() error
    }

    class ValidTicketStatus {
        <<enumeration>>
        active
        used
        cancelled
    }

    ValidTicket --> ValidTicketStatus : has
```

---

## Repository Interfaces

```mermaid
classDiagram
    class EventRepository {
        <<interface>>
        +Get(ctx, id int) *Event, error
        +Add(ctx, event *Event) error
        +Update(ctx, event *Event) error
    }

    class TicketRepository {
        <<interface>>
        +Get(ctx, id int) *Ticket, error
        +GetByCode(ctx, code string) *Ticket, error
        +Add(ctx, ticket *Ticket) error
        +Update(ctx, ticket *Ticket) error
        +FindByPurchaseID(ctx, purchaseID int) []*Ticket, error
        +FindByEventID(ctx, eventID int) []*Ticket, error
    }

    class PurchaseRepository {
        <<interface>>
        +Get(ctx, id int) *Purchase, error
        +Add(ctx, purchase *Purchase) error
    }

    class ValidTicketRepository {
        <<interface>>
        +Get(ctx, id int) *ValidTicket, error
        +GetByCode(ctx, code string) *ValidTicket, error
        +Add(ctx, vt *ValidTicket) error
        +Update(ctx, vt *ValidTicket) error
    }
```

---

## Service Interfaces

```mermaid
classDiagram
    class TicketEventPublisher {
        <<interface>>
        +PublishTicketCreated(ctx, event) error
        +PublishTicketCancelled(ctx, event) error
    }

    class QRGenerator {
        <<interface>>
        +Generate(code string) []byte, error
    }

    class EmailSender {
        <<interface>>
        +SendTicketEmail(ctx, to, eventName, qrImages) error
    }

    class IDGenerator {
        <<interface>>
        +NextPurchaseID(ctx) int, error
        +NextTicketID(ctx) int, error
    }

    class TicketServiceClient {
        <<interface>>
        +GetTicketByCode(ctx, code string) *TicketInfo, error
    }
```

---

## Cross-Context Relationship

```mermaid
graph LR
    subgraph "Ticket Context (Source of Truth)"
        T[Ticket<br/>id, code, eventID, status]
    end

    subgraph "Message Broker"
        E1[ticket.created<br/>TicketID, TicketCode, EventID]
        E2[ticket.cancelled<br/>TicketID, TicketCode, EventID]
    end

    subgraph "Validator Context (Read Cache)"
        VT[ValidTicket<br/>id, code, eventID, status]
    end

    T -->|publish| E1
    T -->|publish| E2
    E1 -->|consume| VT
    E2 -->|consume| VT
```
