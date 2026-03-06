package ticket

import (
	"context"
	"errors"
	"testing"
)

// --- Mocks ---

type mockEventRepository struct {
	getFunc    func(ctx context.Context, id int) (*Event, error)
	addFunc    func(ctx context.Context, event *Event) error
	updateFunc func(ctx context.Context, event *Event) error
}

func (m *mockEventRepository) Get(ctx context.Context, id int) (*Event, error) {
	return m.getFunc(ctx, id)
}

func (m *mockEventRepository) Add(ctx context.Context, event *Event) error {
	if m.addFunc != nil {
		return m.addFunc(ctx, event)
	}

	return nil
}

func (m *mockEventRepository) Update(ctx context.Context, event *Event) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, event)
	}

	return nil
}

type mockTicketRepository struct {
	getFunc            func(ctx context.Context, id int) (*Ticket, error)
	getByCodeFunc      func(ctx context.Context, code string) (*Ticket, error)
	addFunc            func(ctx context.Context, ticket *Ticket) error
	updateFunc         func(ctx context.Context, ticket *Ticket) error
	findByPurchaseFunc func(ctx context.Context, purchaseID int) ([]*Ticket, error)
	findByEventFunc    func(ctx context.Context, eventID int) ([]*Ticket, error)
}

func (m *mockTicketRepository) Get(ctx context.Context, id int) (*Ticket, error) {
	return m.getFunc(ctx, id)
}

func (m *mockTicketRepository) GetByCode(ctx context.Context, code string) (*Ticket, error) {
	if m.getByCodeFunc != nil {
		return m.getByCodeFunc(ctx, code)
	}

	return nil, nil
}

func (m *mockTicketRepository) Add(ctx context.Context, ticket *Ticket) error {
	if m.addFunc != nil {
		return m.addFunc(ctx, ticket)
	}

	return nil
}

func (m *mockTicketRepository) Update(ctx context.Context, ticket *Ticket) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, ticket)
	}

	return nil
}

func (m *mockTicketRepository) FindByPurchaseID(ctx context.Context, purchaseID int) ([]*Ticket, error) {
	if m.findByPurchaseFunc != nil {
		return m.findByPurchaseFunc(ctx, purchaseID)
	}

	return nil, nil
}

func (m *mockTicketRepository) FindByEventID(ctx context.Context, eventID int) ([]*Ticket, error) {
	if m.findByEventFunc != nil {
		return m.findByEventFunc(ctx, eventID)
	}

	return nil, nil
}

type mockPurchaseRepository struct {
	getFunc func(ctx context.Context, id int) (*Purchase, error)
	addFunc func(ctx context.Context, purchase *Purchase) error
}

func (m *mockPurchaseRepository) Get(ctx context.Context, id int) (*Purchase, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}

	return nil, nil
}

func (m *mockPurchaseRepository) Add(ctx context.Context, purchase *Purchase) error {
	if m.addFunc != nil {
		return m.addFunc(ctx, purchase)
	}

	return nil
}

type mockEventPublisher struct {
	publishCreatedCalls   int
	publishCancelledCalls int
	publishCreatedErr     error
	publishCancelledErr   error
}

func (m *mockEventPublisher) PublishTicketCreated(ctx context.Context, event TicketCreatedEvent) error {
	m.publishCreatedCalls++

	return m.publishCreatedErr
}

func (m *mockEventPublisher) PublishTicketCancelled(ctx context.Context, event TicketCancelledEvent) error {
	m.publishCancelledCalls++

	return m.publishCancelledErr
}

func (m *mockEventPublisher) PublishPurchaseCompleted(ctx context.Context, event PurchaseCompletedEvent) error {
	return nil
}

// --- Tests ---

func TestTicketService_Purchase_Success(t *testing.T) {
	event := newTestEvent(t, 100)

	eventRepo := &mockEventRepository{
		getFunc: func(ctx context.Context, id int) (*Event, error) {
			return event, nil
		},
	}

	nextTicketID := 1000
	ticketRepo := &mockTicketRepository{
		getFunc: func(ctx context.Context, id int) (*Ticket, error) {
			return nil, nil
		},
		addFunc: func(ctx context.Context, ticket *Ticket) error {
			_ = ticket.SetID(nextTicketID)
			nextTicketID++

			return nil
		},
	}

	purchaseRepo := &mockPurchaseRepository{
		addFunc: func(ctx context.Context, purchase *Purchase) error {
			return purchase.SetID(1000)
		},
	}
	publisher := &mockEventPublisher{}

	svc := NewTicketService(eventRepo, ticketRepo, purchaseRepo, publisher)

	result, err := svc.Purchase(context.Background(), PurchaseInput{
		BuyerEmail: "user@example.com",
		EventID:    1,
		Quantity:   2,
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.PurchaseID != 1000 {
		t.Errorf("expected purchase ID 1000, got %d", result.PurchaseID)
	}

	if len(result.Tickets) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(result.Tickets))
	}

	if publisher.publishCreatedCalls != 2 {
		t.Errorf("expected 2 publish calls, got %d", publisher.publishCreatedCalls)
	}

	if event.SoldCount() != 2 {
		t.Errorf("expected sold count 2, got %d", event.SoldCount())
	}
}

func TestTicketService_Purchase_EventNotFound(t *testing.T) {
	eventRepo := &mockEventRepository{
		getFunc: func(ctx context.Context, id int) (*Event, error) {
			return nil, nil
		},
	}

	svc := NewTicketService(eventRepo, nil, nil, nil)

	_, err := svc.Purchase(context.Background(), PurchaseInput{
		BuyerEmail: "user@example.com",
		EventID:    999,
		Quantity:   1,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTicketService_Purchase_NotEnoughTickets(t *testing.T) {
	event := newTestEvent(t, 1)

	eventRepo := &mockEventRepository{
		getFunc: func(ctx context.Context, id int) (*Event, error) {
			return event, nil
		},
	}

	svc := NewTicketService(eventRepo, nil, nil, nil)

	_, err := svc.Purchase(context.Background(), PurchaseInput{
		BuyerEmail: "user@example.com",
		EventID:    1,
		Quantity:   5,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrBusinessRule) {
		t.Errorf("expected ErrBusinessRule, got %v", err)
	}
}

func TestTicketService_Purchase_EventRepoError(t *testing.T) {
	eventRepo := &mockEventRepository{
		getFunc: func(ctx context.Context, id int) (*Event, error) {
			return nil, errors.New("db connection error")
		},
	}

	svc := NewTicketService(eventRepo, nil, nil, nil)

	_, err := svc.Purchase(context.Background(), PurchaseInput{
		BuyerEmail: "user@example.com",
		EventID:    1,
		Quantity:   1,
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestTicketService_CancelTicket_Success(t *testing.T) {
	ticket := newTestTicket(t)

	ticketRepo := &mockTicketRepository{
		getFunc: func(ctx context.Context, id int) (*Ticket, error) {
			return ticket, nil
		},
	}

	publisher := &mockEventPublisher{}

	svc := NewTicketService(nil, ticketRepo, nil, publisher)

	err := svc.CancelTicket(context.Background(), 1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if ticket.Status() != TicketStatusCancelled {
		t.Errorf("expected status 'cancelled', got %q", ticket.Status())
	}

	if publisher.publishCancelledCalls != 1 {
		t.Errorf("expected 1 cancelled publish call, got %d", publisher.publishCancelledCalls)
	}
}

func TestTicketService_CancelTicket_NotFound(t *testing.T) {
	ticketRepo := &mockTicketRepository{
		getFunc: func(ctx context.Context, id int) (*Ticket, error) {
			return nil, nil
		},
	}

	svc := NewTicketService(nil, ticketRepo, nil, nil)

	err := svc.CancelTicket(context.Background(), 999)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTicketService_CancelTicket_AlreadyUsed(t *testing.T) {
	ticket := newTestTicket(t)
	_ = ticket.MarkAsUsed()

	ticketRepo := &mockTicketRepository{
		getFunc: func(ctx context.Context, id int) (*Ticket, error) {
			return ticket, nil
		},
	}

	svc := NewTicketService(nil, ticketRepo, nil, nil)

	err := svc.CancelTicket(context.Background(), 1)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrBusinessRule) {
		t.Errorf("expected ErrBusinessRule, got %v", err)
	}
}

func TestTicketService_GetTicketByCode_Success(t *testing.T) {
	expected := newTestTicket(t)

	ticketRepo := &mockTicketRepository{
		getFunc: func(ctx context.Context, id int) (*Ticket, error) {
			return nil, nil
		},
		getByCodeFunc: func(ctx context.Context, code string) (*Ticket, error) {
			return expected, nil
		},
	}

	svc := NewTicketService(nil, ticketRepo, nil, nil)

	result, err := svc.GetTicketByCode(context.Background(), expected.Code())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.ID() != expected.ID() {
		t.Errorf("expected ticket ID %d, got %d", expected.ID(), result.ID())
	}
}

func TestTicketService_GetTicketByCode_NotFound(t *testing.T) {
	ticketRepo := &mockTicketRepository{
		getFunc: func(ctx context.Context, id int) (*Ticket, error) {
			return nil, nil
		},
		getByCodeFunc: func(ctx context.Context, code string) (*Ticket, error) {
			return nil, nil
		},
	}

	svc := NewTicketService(nil, ticketRepo, nil, nil)

	_, err := svc.GetTicketByCode(context.Background(), "nonexistent")

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- SyncTicketUsed Tests ---

func TestTicketService_SyncTicketUsed_Success(t *testing.T) {
	ticket := newTestTicket(t)
	updateCalled := false

	ticketRepo := &mockTicketRepository{
		getFunc: func(ctx context.Context, id int) (*Ticket, error) {
			return nil, nil
		},
		getByCodeFunc: func(ctx context.Context, code string) (*Ticket, error) {
			return ticket, nil
		},
		updateFunc: func(ctx context.Context, t *Ticket) error {
			updateCalled = true

			return nil
		},
	}

	svc := NewTicketService(nil, ticketRepo, nil, nil)

	err := svc.SyncTicketUsed(context.Background(), ticket.Code())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !updateCalled {
		t.Error("expected Update to be called")
	}

	if ticket.Status() != TicketStatusUsed {
		t.Errorf("expected status 'used', got %q", ticket.Status())
	}
}

func TestTicketService_SyncTicketUsed_AlreadyUsed(t *testing.T) {
	ticket := newTestTicket(t)
	_ = ticket.MarkAsUsed()
	updateCalled := false

	ticketRepo := &mockTicketRepository{
		getFunc: func(ctx context.Context, id int) (*Ticket, error) {
			return nil, nil
		},
		getByCodeFunc: func(ctx context.Context, code string) (*Ticket, error) {
			return ticket, nil
		},
		updateFunc: func(ctx context.Context, t *Ticket) error {
			updateCalled = true

			return nil
		},
	}

	svc := NewTicketService(nil, ticketRepo, nil, nil)

	err := svc.SyncTicketUsed(context.Background(), ticket.Code())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if updateCalled {
		t.Error("expected Update NOT to be called (idempotent)")
	}
}

func TestTicketService_SyncTicketUsed_NotFound(t *testing.T) {
	ticketRepo := &mockTicketRepository{
		getFunc: func(ctx context.Context, id int) (*Ticket, error) {
			return nil, nil
		},
		getByCodeFunc: func(ctx context.Context, code string) (*Ticket, error) {
			return nil, nil
		},
	}

	svc := NewTicketService(nil, ticketRepo, nil, nil)

	err := svc.SyncTicketUsed(context.Background(), "nonexistent")

	if err != nil {
		t.Fatalf("expected no error (ignore missing), got %v", err)
	}
}

func TestTicketService_SyncTicketUsed_RepoError(t *testing.T) {
	ticketRepo := &mockTicketRepository{
		getFunc: func(ctx context.Context, id int) (*Ticket, error) {
			return nil, nil
		},
		getByCodeFunc: func(ctx context.Context, code string) (*Ticket, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTicketService(nil, ticketRepo, nil, nil)

	err := svc.SyncTicketUsed(context.Background(), "abc-123")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
