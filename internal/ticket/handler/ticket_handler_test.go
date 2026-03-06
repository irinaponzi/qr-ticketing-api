package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/iponzi/entradasQR/internal/ticket"
)

// --- Mocks ---

type mockEventRepo struct {
	getFunc    func(ctx context.Context, id int) (*ticket.Event, error)
	addFunc    func(ctx context.Context, event *ticket.Event) error
	updateFunc func(ctx context.Context, event *ticket.Event) error
}

func (m *mockEventRepo) Get(ctx context.Context, id int) (*ticket.Event, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}

	return nil, nil
}

func (m *mockEventRepo) Add(ctx context.Context, event *ticket.Event) error {
	if m.addFunc != nil {
		return m.addFunc(ctx, event)
	}

	return nil
}

func (m *mockEventRepo) Update(ctx context.Context, event *ticket.Event) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, event)
	}

	return nil
}

type mockTicketRepo struct {
	getFunc       func(ctx context.Context, id int) (*ticket.Ticket, error)
	getByCodeFunc func(ctx context.Context, code string) (*ticket.Ticket, error)
	addFunc       func(ctx context.Context, t *ticket.Ticket) error
	updateFunc    func(ctx context.Context, t *ticket.Ticket) error
}

func (m *mockTicketRepo) Get(ctx context.Context, id int) (*ticket.Ticket, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}

	return nil, nil
}

func (m *mockTicketRepo) GetByCode(ctx context.Context, code string) (*ticket.Ticket, error) {
	if m.getByCodeFunc != nil {
		return m.getByCodeFunc(ctx, code)
	}

	return nil, nil
}

func (m *mockTicketRepo) Add(ctx context.Context, t *ticket.Ticket) error {
	if m.addFunc != nil {
		return m.addFunc(ctx, t)
	}

	return nil
}

func (m *mockTicketRepo) Update(ctx context.Context, t *ticket.Ticket) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, t)
	}

	return nil
}

func (m *mockTicketRepo) FindByPurchaseID(ctx context.Context, purchaseID int) ([]*ticket.Ticket, error) {
	return nil, nil
}

func (m *mockTicketRepo) FindByEventID(ctx context.Context, eventID int) ([]*ticket.Ticket, error) {
	return nil, nil
}

type mockPurchaseRepo struct {
	addFunc func(ctx context.Context, p *ticket.Purchase) error
}

func (m *mockPurchaseRepo) Get(ctx context.Context, id int) (*ticket.Purchase, error) {
	return nil, nil
}

func (m *mockPurchaseRepo) Add(ctx context.Context, p *ticket.Purchase) error {
	if m.addFunc != nil {
		return m.addFunc(ctx, p)
	}

	return nil
}

type mockPublisher struct{}

func (m *mockPublisher) PublishTicketCreated(ctx context.Context, event ticket.TicketCreatedEvent) error {
	return nil
}

func (m *mockPublisher) PublishTicketCancelled(ctx context.Context, event ticket.TicketCancelledEvent) error {
	return nil
}

func (m *mockPublisher) PublishPurchaseCompleted(ctx context.Context, event ticket.PurchaseCompletedEvent) error {
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// --- Tests ---

func TestCreateEvent_Success(t *testing.T) {
	eventRepo := &mockEventRepo{
		addFunc: func(ctx context.Context, event *ticket.Event) error {
			event.SetID(42)

			return nil
		},
	}
	svc := ticket.NewTicketService(eventRepo, nil, nil, nil)
	handler := NewTicketHandler(svc, eventRepo, testLogger())

	body := `{"name":"Rock Concert","location":"Stadium","date":"2026-06-15T20:00:00Z","capacity":1000,"ticket_price":150.0}`
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.CreateEvent(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	var resp createEventResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != 42 {
		t.Errorf("expected ID 42, got %d", resp.ID)
	}

	if resp.Name != "Rock Concert" {
		t.Errorf("expected name 'Rock Concert', got %q", resp.Name)
	}

	if resp.Capacity != 1000 {
		t.Errorf("expected capacity 1000, got %d", resp.Capacity)
	}
}

func TestCreateEvent_InvalidBody(t *testing.T) {
	handler := NewTicketHandler(nil, nil, testLogger())

	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString("not json"))
	rr := httptest.NewRecorder()

	handler.CreateEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCreateEvent_InvalidDate(t *testing.T) {
	handler := NewTicketHandler(nil, nil, testLogger())

	body := `{"name":"Concert","location":"Venue","date":"not-a-date","capacity":100,"ticket_price":50.0}`
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.CreateEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCreateEvent_RepoError(t *testing.T) {
	eventRepo := &mockEventRepo{
		addFunc: func(ctx context.Context, event *ticket.Event) error {
			return errors.New("db error")
		},
	}

	svc := ticket.NewTicketService(eventRepo, nil, nil, nil)
	handler := NewTicketHandler(svc, eventRepo, testLogger())

	body := `{"name":"Concert","location":"Venue","date":"2026-06-15T20:00:00Z","capacity":100,"ticket_price":50.0}`
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.CreateEvent(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestCreatePurchase_Success(t *testing.T) {
	date := time.Date(2026, 6, 15, 20, 0, 0, 0, time.UTC)
	event, _ := ticket.NewEvent(1, "Concert", "Venue", date, 100, 150.0)

	eventRepo := &mockEventRepo{
		getFunc: func(ctx context.Context, id int) (*ticket.Event, error) {
			return event, nil
		},
	}

	nextTicketID := 1000
	ticketRepo := &mockTicketRepo{
		addFunc: func(ctx context.Context, t *ticket.Ticket) error {
			_ = t.SetID(nextTicketID)
			nextTicketID++

			return nil
		},
	}
	purchaseRepo := &mockPurchaseRepo{
		addFunc: func(ctx context.Context, p *ticket.Purchase) error {
			return p.SetID(1000)
		},
	}
	publisher := &mockPublisher{}

	svc := ticket.NewTicketService(eventRepo, ticketRepo, purchaseRepo, publisher)
	handler := NewTicketHandler(svc, eventRepo, testLogger())

	body := `{"buyer_email":"user@example.com","event_id":1,"quantity":2}`
	req := httptest.NewRequest(http.MethodPost, "/purchases", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.CreatePurchase(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var resp createPurchaseResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.PurchaseID != 1000 {
		t.Errorf("expected purchase ID 1000, got %d", resp.PurchaseID)
	}

	if len(resp.Tickets) != 2 {
		t.Errorf("expected 2 tickets, got %d", len(resp.Tickets))
	}
}

func TestCreatePurchase_EventNotFound(t *testing.T) {
	eventRepo := &mockEventRepo{
		getFunc: func(ctx context.Context, id int) (*ticket.Event, error) {
			return nil, nil
		},
	}

	svc := ticket.NewTicketService(eventRepo, nil, nil, nil)
	handler := NewTicketHandler(svc, eventRepo, testLogger())

	body := `{"buyer_email":"user@example.com","event_id":999,"quantity":1}`
	req := httptest.NewRequest(http.MethodPost, "/purchases", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.CreatePurchase(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestLookupTicket_Success(t *testing.T) {
	tk, _ := ticket.NewTicket(1, 10, 100)

	ticketRepo := &mockTicketRepo{
		getByCodeFunc: func(ctx context.Context, code string) (*ticket.Ticket, error) {
			return tk, nil
		},
	}

	svc := ticket.NewTicketService(nil, ticketRepo, nil, nil)
	handler := NewTicketHandler(svc, nil, testLogger())

	body := `{"code":"` + tk.Code() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/tickets/lookup", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.LookupTicket(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var resp ticketResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Code != tk.Code() {
		t.Errorf("expected code %q, got %q", tk.Code(), resp.Code)
	}
}

func TestLookupTicket_EmptyCode(t *testing.T) {
	handler := NewTicketHandler(nil, nil, testLogger())

	body := `{"code":""}`
	req := httptest.NewRequest(http.MethodPost, "/tickets/lookup", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.LookupTicket(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestLookupTicket_NotFound(t *testing.T) {
	ticketRepo := &mockTicketRepo{
		getByCodeFunc: func(ctx context.Context, code string) (*ticket.Ticket, error) {
			return nil, nil
		},
	}

	svc := ticket.NewTicketService(nil, ticketRepo, nil, nil)
	handler := NewTicketHandler(svc, nil, testLogger())

	body := `{"code":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/tickets/lookup", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.LookupTicket(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestCancelTicket_Success(t *testing.T) {
	tk, _ := ticket.NewTicket(1, 10, 100)

	ticketRepo := &mockTicketRepo{
		getFunc: func(ctx context.Context, id int) (*ticket.Ticket, error) {
			return tk, nil
		},
	}

	publisher := &mockPublisher{}
	svc := ticket.NewTicketService(nil, ticketRepo, nil, publisher)
	handler := NewTicketHandler(svc, nil, testLogger())

	body := `{"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/tickets/cancel", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.CancelTicket(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d — body: %s", rr.Code, rr.Body.String())
	}
}

func TestCancelTicket_InvalidID(t *testing.T) {
	handler := NewTicketHandler(nil, nil, testLogger())

	body := `{"id":0}`
	req := httptest.NewRequest(http.MethodPost, "/tickets/cancel", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.CancelTicket(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestCancelTicket_NotFound(t *testing.T) {
	ticketRepo := &mockTicketRepo{
		getFunc: func(ctx context.Context, id int) (*ticket.Ticket, error) {
			return nil, nil
		},
	}

	svc := ticket.NewTicketService(nil, ticketRepo, nil, nil)
	handler := NewTicketHandler(svc, nil, testLogger())

	body := `{"id":999}`
	req := httptest.NewRequest(http.MethodPost, "/tickets/cancel", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	handler.CancelTicket(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}
