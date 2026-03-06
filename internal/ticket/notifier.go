package ticket

import "context"

// TicketCreatedEvent carries the information needed to notify other systems
// about a new valid ticket.
type TicketCreatedEvent struct {
	TicketID   int
	TicketCode string
	EventID    int
}

// TicketCancelledEvent carries the information needed to notify other systems
// about a cancelled ticket.
type TicketCancelledEvent struct {
	TicketID   int
	TicketCode string
	EventID    int
}

// PurchaseCompletedEvent carries all data needed by downstream workers
// (QR generation, email delivery) after a successful purchase.
type PurchaseCompletedEvent struct {
	PurchaseID  int
	BuyerEmail  string
	EventName   string
	TicketCodes []string
}

// TicketEventPublisher publishes ticket lifecycle events to a message broker.
type TicketEventPublisher interface {
	PublishTicketCreated(ctx context.Context, event TicketCreatedEvent) error
	PublishTicketCancelled(ctx context.Context, event TicketCancelledEvent) error
	PublishPurchaseCompleted(ctx context.Context, event PurchaseCompletedEvent) error
}

// TokenSigner signs and verifies ticket codes using a cryptographic scheme.
type TokenSigner interface {
	Sign(code string) string
	Verify(token string) (code string, valid bool)
}

// QRGenerator generates QR code images from ticket codes.
type QRGenerator interface {
	Generate(code string) ([]byte, error)
}

// EmailSender sends emails to buyers with their ticket information.
type EmailSender interface {
	SendTicketEmail(ctx context.Context, to string, eventName string, qrImages [][]byte) error
}
