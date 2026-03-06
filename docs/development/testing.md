# Testing Strategy

Comprehensive testing approach covering domain entities, services, handlers, and repositories.

---

## Test Distribution

| Layer | Package | Tests | Approach |
|---|---|---|---|
| **Entities** | `internal/ticket/` | 20+ | Pure unit tests, no mocks |
| **Entities** | `internal/validator/` | 10+ | Pure unit tests, no mocks |
| **Services** | `internal/ticket/` | 10+ | Manual mocks for all ports |
| **Services** | `internal/validator/` | 12+ | Manual mocks for all ports |
| **Handlers** | `internal/ticket/handler/` | 11 | `httptest` + manual mocks |
| **Handlers** | `internal/validator/handler/` | 6 | `httptest` + manual mocks |
| **Repositories** | `internal/ticket/storage/` | 11 | `sqlmock` for MySQL |
| **Repositories** | `internal/validator/storage/` | 8 | `sqlmock` for MySQL |

**Total: 65+ tests**

---

## Running Tests

```bash
# All tests with verbose output
make test

# With coverage report
make test-cover

# Specific package
go test ./internal/ticket/... -v -count=1

# Single test
go test ./internal/ticket/ -run TestPurchase_Success -v
```

---

## Testing Patterns

### 1. Entity Tests (Pure Domain)

No dependencies, no mocks. Test invariants, state transitions, and validation.

```go
func TestNewEvent_InvalidCapacity(t *testing.T) {
    _, err := NewEvent(1, "Concert", "Venue", futureDate, 0)
    if err == nil {
        t.Error("expected error for zero capacity")
    }
}

func TestTicket_Cancel_AlreadyUsed(t *testing.T) {
    tk, _ := NewTicket(1, 10, 100)
    _ = tk.MarkAsUsed()

    err := tk.Cancel()
    if err == nil {
        t.Error("expected error cancelling used ticket")
    }
}
```

### 2. Service Tests (with Manual Mocks)

Each port (repository, publisher, etc.) has a manual mock implementing the interface:

```go
type mockEventRepository struct {
    getFunc    func(ctx context.Context, id int) (*Event, error)
    addFunc    func(ctx context.Context, event *Event) error
    updateFunc func(ctx context.Context, event *Event) error
}

func (m *mockEventRepository) Get(ctx context.Context, id int) (*Event, error) {
    return m.getFunc(ctx, id)
}
```

Tests inject specific behavior per test case:

```go
func TestPurchase_EventNotFound(t *testing.T) {
    eventRepo := &mockEventRepository{
        getFunc: func(ctx context.Context, id int) (*Event, error) {
            return nil, nil  // not found
        },
    }
    svc := NewTicketService(eventRepo, ...)
    _, err := svc.Purchase(ctx, input)
    // assert ErrNotFound
}
```

#### Simulating Database-Assigned IDs

Repository `Add` methods assign the database-generated ID via `SetID()`. Mocks must replicate this behavior to test that the caller receives the correct ID:

```go
eventRepo := &mockEventRepo{
    addFunc: func(ctx context.Context, event *ticket.Event) error {
        event.SetID(42) // simulate DB auto-increment
        return nil
    },
}
```

This pattern applies to `Event`, `Ticket`, and `Purchase` entities.

### 3. Handler Tests (httptest)

Use `net/http/httptest` to test HTTP request/response cycle:

```go
func TestCreateEvent_InvalidBody(t *testing.T) {
    handler := NewTicketHandler(nil, nil, testLogger())
    req := httptest.NewRequest(http.MethodPost, "/events",
        bytes.NewBufferString("not json"))
    rr := httptest.NewRecorder()

    handler.CreateEvent(rr, req)

    if rr.Code != http.StatusBadRequest {
        t.Errorf("expected 400, got %d", rr.Code)
    }
}
```

For endpoints using chi URL params, inject a route context:

```go
rctx := chi.NewRouteContext()
rctx.URLParams.Add("code", "abc-123")
req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
```

### 4. Repository Tests (sqlmock)

Use `github.com/DATA-DOG/go-sqlmock` to test SQL queries without a real database:

```go
func TestMySQLEventRepository_Get_Success(t *testing.T) {
    db, mock, err := sqlmock.New()
    // ...
    rows := sqlmock.NewRows(columns).AddRow(values...)
    mock.ExpectQuery("SELECT .+ FROM events").
        WithArgs(1).
        WillReturnRows(rows)

    repo := NewMySQLEventRepository(db)
    event, err := repo.Get(ctx, 1)
    // assert event fields
    // assert mock expectations met
}
```

---

## Test Coverage

```bash
make test-cover
```

Generates `coverage.out` with per-function coverage. View HTML report:

```bash
go tool cover -html=coverage.out
```

### Coverage Targets

| Package | Target |
|---|---|
| Domain entities | > 90% |
| Domain services | > 80% |
| Handlers | > 70% |
| Repositories | > 70% |

---

## Why Manual Mocks?

This project uses **manual mock structs** instead of code generation tools (e.g., gomock, mockery) because:

1. **Simplicity** — No extra tooling or generated files
2. **Readability** — Mocks are in the same test file, easy to understand
3. **Flexibility** — Each test defines exact mock behavior via function fields
4. **Small surface** — Interfaces have 2-4 methods max, manual mocks are trivial

For larger projects, consider `go generate` + mockery for automatic mock generation.
