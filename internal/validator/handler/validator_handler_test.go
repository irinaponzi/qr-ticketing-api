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
	"strings"
	"testing"

	"github.com/iponzi/entradasQR/internal/validator"
)

// --- Mocks ---

type mockValidTicketRepo struct {
	getByCodeFunc func(ctx context.Context, code string) (*validator.ValidTicket, error)
	addFunc       func(ctx context.Context, ticket *validator.ValidTicket) error
	updateFunc    func(ctx context.Context, ticket *validator.ValidTicket) error
}

func (m *mockValidTicketRepo) Get(ctx context.Context, id int) (*validator.ValidTicket, error) {
	return nil, nil
}

func (m *mockValidTicketRepo) GetByCode(ctx context.Context, code string) (*validator.ValidTicket, error) {
	if m.getByCodeFunc != nil {
		return m.getByCodeFunc(ctx, code)
	}

	return nil, nil
}

func (m *mockValidTicketRepo) Add(ctx context.Context, ticket *validator.ValidTicket) error {
	if m.addFunc != nil {
		return m.addFunc(ctx, ticket)
	}

	return nil
}

func (m *mockValidTicketRepo) Update(ctx context.Context, ticket *validator.ValidTicket) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, ticket)
	}

	return nil
}

type mockTicketServiceClient struct {
	getByCodeFunc func(ctx context.Context, code string) (*validator.TicketInfo, error)
}

func (m *mockTicketServiceClient) GetTicketByCode(ctx context.Context, code string) (*validator.TicketInfo, error) {
	if m.getByCodeFunc != nil {
		return m.getByCodeFunc(ctx, code)
	}

	return nil, nil
}

type mockTokenSigner struct{}

func (m *mockTokenSigner) Sign(code string) string {
	return code + ".test-sig"
}

func (m *mockTokenSigner) Verify(token string) (string, bool) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 || parts[1] != "test-sig" {
		return "", false
	}

	return parts[0], true
}

type mockValidatorEventPublisher struct{}

func (m *mockValidatorEventPublisher) PublishTicketUsed(_ context.Context, _ validator.TicketUsedEvent) error {
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// --- Tests ---

func TestValidateTicket_Success(t *testing.T) {
	vt, _ := validator.NewValidTicket(1, "abc-123", 10)

	repo := &mockValidTicketRepo{
		getByCodeFunc: func(ctx context.Context, code string) (*validator.ValidTicket, error) {
			return vt, nil
		},
	}

	svc := validator.NewValidatorService(repo, nil, &mockValidatorEventPublisher{}, testLogger())
	signer := &mockTokenSigner{}
	h := NewValidatorHandler(svc, signer, testLogger())

	body := `{"ticket_code":"abc-123.test-sig"}`
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ValidateTicket(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var resp validateResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if !resp.Valid {
		t.Error("expected valid=true")
	}
}

func TestValidateTicket_AlreadyUsed(t *testing.T) {
	vt, _ := validator.NewValidTicket(1, "abc-123", 10)
	_ = vt.MarkAsUsed()

	repo := &mockValidTicketRepo{
		getByCodeFunc: func(ctx context.Context, code string) (*validator.ValidTicket, error) {
			return vt, nil
		},
	}

	svc := validator.NewValidatorService(repo, nil, &mockValidatorEventPublisher{}, testLogger())
	signer := &mockTokenSigner{}
	h := NewValidatorHandler(svc, signer, testLogger())

	body := `{"ticket_code":"abc-123.test-sig"}`
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	h.ValidateTicket(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp validateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Valid {
		t.Error("expected valid=false for used ticket")
	}
}

func TestValidateTicket_InvalidBody(t *testing.T) {
	h := NewValidatorHandler(nil, &mockTokenSigner{}, testLogger())

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBufferString("not json"))
	rr := httptest.NewRecorder()

	h.ValidateTicket(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestValidateTicket_EmptyCode(t *testing.T) {
	h := NewValidatorHandler(nil, &mockTokenSigner{}, testLogger())

	body := `{"ticket_code":""}`
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	h.ValidateTicket(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestValidateTicket_RepoError(t *testing.T) {
	repo := &mockValidTicketRepo{
		getByCodeFunc: func(ctx context.Context, code string) (*validator.ValidTicket, error) {
			return nil, errors.New("db error")
		},
	}

	svc := validator.NewValidatorService(repo, nil, &mockValidatorEventPublisher{}, testLogger())
	signer := &mockTokenSigner{}
	h := NewValidatorHandler(svc, signer, testLogger())

	body := `{"ticket_code":"abc-123.test-sig"}`
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	h.ValidateTicket(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rr.Code)
	}
}

func TestValidateTicket_FallbackNotFound(t *testing.T) {
	repo := &mockValidTicketRepo{}

	client := &mockTicketServiceClient{
		getByCodeFunc: func(ctx context.Context, code string) (*validator.TicketInfo, error) {
			return nil, nil
		},
	}

	svc := validator.NewValidatorService(repo, client, &mockValidatorEventPublisher{}, testLogger())
	signer := &mockTokenSigner{}
	h := NewValidatorHandler(svc, signer, testLogger())

	body := `{"ticket_code":"nonexistent.test-sig"}`
	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	h.ValidateTicket(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp validateResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Valid {
		t.Error("expected valid=false")
	}

	if resp.Message != "ticket not found" {
		t.Errorf("expected 'ticket not found', got %q", resp.Message)
	}
}
