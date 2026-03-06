package validator

import "context"

// ValidTicketRepository provides access to the collection of valid tickets
// known to the validator service. This is the validator's own database.
// Returns nil for not-found; errors only for infrastructure issues.
type ValidTicketRepository interface {
	Get(ctx context.Context, id int) (*ValidTicket, error)
	GetByCode(ctx context.Context, code string) (*ValidTicket, error)
	Add(ctx context.Context, ticket *ValidTicket) error
	Update(ctx context.Context, ticket *ValidTicket) error
}
