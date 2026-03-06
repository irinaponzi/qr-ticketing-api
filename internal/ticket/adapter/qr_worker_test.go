package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/iponzi/entradasQR/internal/ticket"
	amqp "github.com/rabbitmq/amqp091-go"
)

// --- Mocks ---

type mockQRGenerator struct {
	generateFunc func(code string) ([]byte, error)
}

func (m *mockQRGenerator) Generate(code string) ([]byte, error) {
	if m.generateFunc != nil {
		return m.generateFunc(code)
	}

	return []byte("fake-qr-" + code), nil
}

type mockEmailSender struct {
	sendFunc func(ctx context.Context, to string, eventName string, qrImages [][]byte) error
	calls    int
}

func (m *mockEmailSender) SendTicketEmail(ctx context.Context, to string, eventName string, qrImages [][]byte) error {
	m.calls++

	if m.sendFunc != nil {
		return m.sendFunc(ctx, to, eventName, qrImages)
	}

	return nil
}

type mockTokenSigner struct{}

func (m *mockTokenSigner) Sign(code string) string {
	return code + ".test-signature"
}

func (m *mockTokenSigner) Verify(token string) (string, bool) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 || parts[1] != "test-signature" {
		return "", false
	}

	return parts[0], true
}

// --- Test Helpers ---

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func buildDelivery(t *testing.T, event ticket.PurchaseCompletedEvent) amqp.Delivery {
	t.Helper()

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	return amqp.Delivery{
		Body: body,
	}
}

func newTestEvent() ticket.PurchaseCompletedEvent {
	return ticket.PurchaseCompletedEvent{
		PurchaseID:  1,
		BuyerEmail:  "fan@example.com",
		EventName:   "Rock Festival 2026",
		TicketCodes: []string{"code-aaa", "code-bbb"},
	}
}

// --- Tests ---

func TestQRWorkerConsumer_HandleMessage_Success(t *testing.T) {
	qrGen := &mockQRGenerator{}
	emailSender := &mockEmailSender{}

	consumer := NewQRWorkerConsumer(nil, qrGen, emailSender, &mockTokenSigner{}, testLogger())

	event := newTestEvent()
	msg := buildDelivery(t, event)

	consumer.HandleMessage(context.Background(), msg)

	if emailSender.calls != 1 {
		t.Errorf("expected 1 email send call, got %d", emailSender.calls)
	}
}

func TestQRWorkerConsumer_HandleMessage_InvalidJSON(t *testing.T) {
	qrGen := &mockQRGenerator{}
	emailSender := &mockEmailSender{}

	consumer := NewQRWorkerConsumer(nil, qrGen, emailSender, &mockTokenSigner{}, testLogger())

	msg := amqp.Delivery{
		Body: []byte("not valid json{{{"),
	}

	consumer.HandleMessage(context.Background(), msg)

	if emailSender.calls != 0 {
		t.Errorf("expected 0 email send calls for invalid JSON, got %d", emailSender.calls)
	}
}

func TestQRWorkerConsumer_HandleMessage_QRGenerationError(t *testing.T) {
	failOnSecond := 0
	qrGen := &mockQRGenerator{
		generateFunc: func(code string) ([]byte, error) {
			failOnSecond++
			if failOnSecond == 2 {
				return nil, errors.New("qr generation failed")
			}

			return []byte("qr-" + code), nil
		},
	}

	var receivedImages [][]byte

	emailSender := &mockEmailSender{
		sendFunc: func(ctx context.Context, to string, eventName string, qrImages [][]byte) error {
			receivedImages = qrImages
			return nil
		},
	}

	consumer := NewQRWorkerConsumer(nil, qrGen, emailSender, &mockTokenSigner{}, testLogger())

	event := newTestEvent()
	msg := buildDelivery(t, event)

	consumer.HandleMessage(context.Background(), msg)

	if emailSender.calls != 1 {
		t.Errorf("expected 1 email send call, got %d", emailSender.calls)
	}

	// One QR failed, so only 1 image should be passed to email sender.
	if len(receivedImages) != 1 {
		t.Errorf("expected 1 QR image (one failed), got %d", len(receivedImages))
	}
}

func TestQRWorkerConsumer_HandleMessage_AllQRsFail(t *testing.T) {
	qrGen := &mockQRGenerator{
		generateFunc: func(code string) ([]byte, error) {
			return nil, errors.New("qr generation failed")
		},
	}

	var receivedImages [][]byte

	emailSender := &mockEmailSender{
		sendFunc: func(ctx context.Context, to string, eventName string, qrImages [][]byte) error {
			receivedImages = qrImages
			return nil
		},
	}

	consumer := NewQRWorkerConsumer(nil, qrGen, emailSender, &mockTokenSigner{}, testLogger())

	event := newTestEvent()
	msg := buildDelivery(t, event)

	consumer.HandleMessage(context.Background(), msg)

	// Email is still sent even with 0 images (graceful degradation).
	if emailSender.calls != 1 {
		t.Errorf("expected 1 email send call, got %d", emailSender.calls)
	}

	if len(receivedImages) != 0 {
		t.Errorf("expected 0 QR images (all failed), got %d", len(receivedImages))
	}
}

func TestQRWorkerConsumer_HandleMessage_EmailError(t *testing.T) {
	qrGen := &mockQRGenerator{}
	emailSender := &mockEmailSender{
		sendFunc: func(ctx context.Context, to string, eventName string, qrImages [][]byte) error {
			return errors.New("smtp connection refused")
		},
	}

	consumer := NewQRWorkerConsumer(nil, qrGen, emailSender, &mockTokenSigner{}, testLogger())

	event := newTestEvent()
	msg := buildDelivery(t, event)

	consumer.HandleMessage(context.Background(), msg)

	if emailSender.calls != 1 {
		t.Errorf("expected 1 email send call (even with error), got %d", emailSender.calls)
	}
}

func TestQRWorkerConsumer_HandleMessage_CorrectEmailParams(t *testing.T) {
	qrGen := &mockQRGenerator{}

	var capturedTo, capturedEventName string
	var capturedQRCount int

	emailSender := &mockEmailSender{
		sendFunc: func(ctx context.Context, to string, eventName string, qrImages [][]byte) error {
			capturedTo = to
			capturedEventName = eventName
			capturedQRCount = len(qrImages)

			return nil
		},
	}

	consumer := NewQRWorkerConsumer(nil, qrGen, emailSender, &mockTokenSigner{}, testLogger())

	event := ticket.PurchaseCompletedEvent{
		PurchaseID:  42,
		BuyerEmail:  "vip@example.com",
		EventName:   "Jazz Night",
		TicketCodes: []string{"t1", "t2", "t3"},
	}
	msg := buildDelivery(t, event)

	consumer.HandleMessage(context.Background(), msg)

	if capturedTo != "vip@example.com" {
		t.Errorf("expected email to 'vip@example.com', got %q", capturedTo)
	}

	if capturedEventName != "Jazz Night" {
		t.Errorf("expected event name 'Jazz Night', got %q", capturedEventName)
	}

	if capturedQRCount != 3 {
		t.Errorf("expected 3 QR images, got %d", capturedQRCount)
	}
}

func TestQRWorkerConsumer_HandleMessage_EmptyTicketCodes(t *testing.T) {
	qrGen := &mockQRGenerator{}

	var receivedImages [][]byte

	emailSender := &mockEmailSender{
		sendFunc: func(ctx context.Context, to string, eventName string, qrImages [][]byte) error {
			receivedImages = qrImages
			return nil
		},
	}

	consumer := NewQRWorkerConsumer(nil, qrGen, emailSender, &mockTokenSigner{}, testLogger())

	event := ticket.PurchaseCompletedEvent{
		PurchaseID:  1,
		BuyerEmail:  "fan@example.com",
		EventName:   "Empty Event",
		TicketCodes: []string{},
	}
	msg := buildDelivery(t, event)

	consumer.HandleMessage(context.Background(), msg)

	if emailSender.calls != 1 {
		t.Errorf("expected 1 email send call, got %d", emailSender.calls)
	}

	if len(receivedImages) != 0 {
		t.Errorf("expected 0 QR images for empty ticket codes, got %d", len(receivedImages))
	}
}

func TestQRWorkerConsumer_HandleMessage_SingleTicket(t *testing.T) {
	var generatedCodes []string

	qrGen := &mockQRGenerator{
		generateFunc: func(code string) ([]byte, error) {
			generatedCodes = append(generatedCodes, code)
			return []byte("qr-" + code), nil
		},
	}
	emailSender := &mockEmailSender{}

	consumer := NewQRWorkerConsumer(nil, qrGen, emailSender, &mockTokenSigner{}, testLogger())

	event := ticket.PurchaseCompletedEvent{
		PurchaseID:  5,
		BuyerEmail:  "solo@example.com",
		EventName:   "Solo Show",
		TicketCodes: []string{"only-ticket"},
	}
	msg := buildDelivery(t, event)

	consumer.HandleMessage(context.Background(), msg)

	if len(generatedCodes) != 1 {
		t.Fatalf("expected 1 QR generation call, got %d", len(generatedCodes))
	}

	if generatedCodes[0] != "only-ticket.test-signature" {
		t.Errorf("expected signed token 'only-ticket.test-signature', got %q", generatedCodes[0])
	}
}
