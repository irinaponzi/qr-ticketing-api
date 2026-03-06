package validator

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
)

// --- Mocks ---

type mockValidTicketRepository struct {
	getFunc       func(ctx context.Context, id int) (*ValidTicket, error)
	getByCodeFunc func(ctx context.Context, code string) (*ValidTicket, error)
	addFunc       func(ctx context.Context, ticket *ValidTicket) error
	updateFunc    func(ctx context.Context, ticket *ValidTicket) error
}

func (m *mockValidTicketRepository) Get(ctx context.Context, id int) (*ValidTicket, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}

	return nil, nil
}

func (m *mockValidTicketRepository) GetByCode(ctx context.Context, code string) (*ValidTicket, error) {
	if m.getByCodeFunc != nil {
		return m.getByCodeFunc(ctx, code)
	}

	return nil, nil
}

func (m *mockValidTicketRepository) Add(ctx context.Context, ticket *ValidTicket) error {
	if m.addFunc != nil {
		return m.addFunc(ctx, ticket)
	}

	return nil
}

func (m *mockValidTicketRepository) Update(ctx context.Context, ticket *ValidTicket) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, ticket)
	}

	return nil
}

type mockTicketServiceClient struct {
	getByCodeFunc func(ctx context.Context, code string) (*TicketInfo, error)
}

func (m *mockTicketServiceClient) GetTicketByCode(ctx context.Context, code string) (*TicketInfo, error) {
	if m.getByCodeFunc != nil {
		return m.getByCodeFunc(ctx, code)
	}

	return nil, nil
}

type mockValidatorEventPublisher struct {
	publishFunc func(ctx context.Context, event TicketUsedEvent) error
	calls       []TicketUsedEvent
}

func (m *mockValidatorEventPublisher) PublishTicketUsed(ctx context.Context, event TicketUsedEvent) error {
	m.calls = append(m.calls, event)

	if m.publishFunc != nil {
		return m.publishFunc(ctx, event)
	}

	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// --- ValidateTicket Tests ---

func TestValidatorService_ValidateTicket_LocalActive(t *testing.T) {
	vt, _ := NewValidTicket(1, "abc-123", 10)

	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return vt, nil
		},
	}

	pub := &mockValidatorEventPublisher{}
	svc := NewValidatorService(repo, nil, pub, testLogger())

	result, err := svc.ValidateTicket(context.Background(), "abc-123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !result.Valid {
		t.Error("expected valid=true")
	}

	if result.Message != "ticket validated successfully" {
		t.Errorf("expected success message, got %q", result.Message)
	}

	if vt.Status() != ValidTicketStatusUsed {
		t.Errorf("expected status 'used', got %q", vt.Status())
	}

	if len(pub.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(pub.calls))
	}

	if pub.calls[0].TicketCode != "abc-123" {
		t.Errorf("expected ticket code 'abc-123', got %q", pub.calls[0].TicketCode)
	}
}

func TestValidatorService_ValidateTicket_LocalAlreadyUsed(t *testing.T) {
	vt, _ := NewValidTicket(1, "abc-123", 10)
	_ = vt.MarkAsUsed()

	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return vt, nil
		},
	}

	svc := NewValidatorService(repo, nil, &mockValidatorEventPublisher{}, testLogger())

	result, err := svc.ValidateTicket(context.Background(), "abc-123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for used ticket")
	}

	if result.Message != "ticket is used" {
		t.Errorf("expected 'ticket is used', got %q", result.Message)
	}
}

func TestValidatorService_ValidateTicket_LocalCancelled(t *testing.T) {
	vt, _ := NewValidTicket(1, "abc-123", 10)
	_ = vt.MarkAsCancelled()

	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return vt, nil
		},
	}

	svc := NewValidatorService(repo, nil, &mockValidatorEventPublisher{}, testLogger())

	result, err := svc.ValidateTicket(context.Background(), "abc-123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for cancelled ticket")
	}

	if result.Message != "ticket is cancelled" {
		t.Errorf("expected 'ticket is cancelled', got %q", result.Message)
	}
}

func TestValidatorService_ValidateTicket_FallbackSuccess(t *testing.T) {
	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return nil, nil
		},
	}

	client := &mockTicketServiceClient{
		getByCodeFunc: func(ctx context.Context, code string) (*TicketInfo, error) {
			return &TicketInfo{
				Code:    "abc-123",
				EventID: 10,
				Status:  "emitted",
			}, nil
		},
	}

	svc := NewValidatorService(repo, client, &mockValidatorEventPublisher{}, testLogger())

	result, err := svc.ValidateTicket(context.Background(), "abc-123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !result.Valid {
		t.Error("expected valid=true from fallback")
	}

	if result.Message != "ticket validated via live check" {
		t.Errorf("expected fallback message, got %q", result.Message)
	}
}

func TestValidatorService_ValidateTicket_FallbackNotFound(t *testing.T) {
	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return nil, nil
		},
	}

	client := &mockTicketServiceClient{
		getByCodeFunc: func(ctx context.Context, code string) (*TicketInfo, error) {
			return nil, nil
		},
	}

	svc := NewValidatorService(repo, client, &mockValidatorEventPublisher{}, testLogger())

	result, err := svc.ValidateTicket(context.Background(), "nonexistent")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false")
	}

	if result.Message != "ticket not found" {
		t.Errorf("expected 'ticket not found', got %q", result.Message)
	}
}

func TestValidatorService_ValidateTicket_FallbackUsedTicket(t *testing.T) {
	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return nil, nil
		},
	}

	client := &mockTicketServiceClient{
		getByCodeFunc: func(ctx context.Context, code string) (*TicketInfo, error) {
			return &TicketInfo{
				Code:    "abc-123",
				EventID: 10,
				Status:  "used",
			}, nil
		},
	}

	svc := NewValidatorService(repo, client, &mockValidatorEventPublisher{}, testLogger())

	result, err := svc.ValidateTicket(context.Background(), "abc-123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for used ticket")
	}
}

func TestValidatorService_ValidateTicket_FallbackServiceError(t *testing.T) {
	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return nil, nil
		},
	}

	client := &mockTicketServiceClient{
		getByCodeFunc: func(ctx context.Context, code string) (*TicketInfo, error) {
			return nil, errors.New("connection refused")
		},
	}

	svc := NewValidatorService(repo, client, &mockValidatorEventPublisher{}, testLogger())

	result, err := svc.ValidateTicket(context.Background(), "abc-123")

	if err != nil {
		t.Fatalf("expected no error (graceful degradation), got %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false on service error")
	}

	if result.Message != "unable to validate ticket at this time" {
		t.Errorf("expected degradation message, got %q", result.Message)
	}
}

func TestValidatorService_ValidateTicket_RepoError(t *testing.T) {
	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewValidatorService(repo, nil, &mockValidatorEventPublisher{}, testLogger())

	_, err := svc.ValidateTicket(context.Background(), "abc-123")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- SyncTicketCreated Tests ---

func TestValidatorService_SyncTicketCreated_Success(t *testing.T) {
	addCalled := false

	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return nil, nil
		},
		addFunc: func(ctx context.Context, ticket *ValidTicket) error {
			addCalled = true

			return nil
		},
	}

	svc := NewValidatorService(repo, nil, &mockValidatorEventPublisher{}, testLogger())

	err := svc.SyncTicketCreated(context.Background(), "abc-123", 10)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !addCalled {
		t.Error("expected Add to be called")
	}
}

func TestValidatorService_SyncTicketCreated_Idempotent(t *testing.T) {
	vt, _ := NewValidTicket(1, "abc-123", 10)
	addCalled := false

	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return vt, nil
		},
		addFunc: func(ctx context.Context, ticket *ValidTicket) error {
			addCalled = true

			return nil
		},
	}

	svc := NewValidatorService(repo, nil, &mockValidatorEventPublisher{}, testLogger())

	err := svc.SyncTicketCreated(context.Background(), "abc-123", 10)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if addCalled {
		t.Error("expected Add NOT to be called (idempotent)")
	}
}

// --- SyncTicketCancelled Tests ---

func TestValidatorService_SyncTicketCancelled_Success(t *testing.T) {
	vt, _ := NewValidTicket(1, "abc-123", 10)
	updateCalled := false

	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return vt, nil
		},
		updateFunc: func(ctx context.Context, ticket *ValidTicket) error {
			updateCalled = true

			return nil
		},
	}

	svc := NewValidatorService(repo, nil, &mockValidatorEventPublisher{}, testLogger())

	err := svc.SyncTicketCancelled(context.Background(), "abc-123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !updateCalled {
		t.Error("expected Update to be called")
	}

	if vt.Status() != ValidTicketStatusCancelled {
		t.Errorf("expected status 'cancelled', got %q", vt.Status())
	}
}

func TestValidatorService_SyncTicketCancelled_NotFound(t *testing.T) {
	repo := &mockValidTicketRepository{
		getByCodeFunc: func(ctx context.Context, code string) (*ValidTicket, error) {
			return nil, nil
		},
	}

	svc := NewValidatorService(repo, nil, &mockValidatorEventPublisher{}, testLogger())

	err := svc.SyncTicketCancelled(context.Background(), "nonexistent")

	if err != nil {
		t.Fatalf("expected no error (ignore), got %v", err)
	}
}
