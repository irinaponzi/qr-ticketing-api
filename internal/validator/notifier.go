package validator

import "context"

// TicketUsedEvent carries the information needed to notify the Ticket Service
// that a ticket has been validated and marked as used at the venue.
// This enables bidirectional reconciliation of ticket state.
type TicketUsedEvent struct {
	TicketCode string
	EventID    int
}

// ValidatorEventPublisher publishes validation events to a message broker
// so the Ticket Service can reconcile ticket state.
type ValidatorEventPublisher interface {
	PublishTicketUsed(ctx context.Context, event TicketUsedEvent) error
}

// TokenSigner signs and verifies ticket codes using a cryptographic scheme.
type TokenSigner interface {
	Sign(code string) string
	Verify(token string) (code string, valid bool)
}
