package ticket

import "context"

// EventRepository provides access to the collection of events.
// Returns nil for not-found; errors only for infrastructure issues.
type EventRepository interface {
	Get(ctx context.Context, id int) (*Event, error)
	Add(ctx context.Context, event *Event) error
	Update(ctx context.Context, event *Event) error
}

// TicketRepository provides access to the collection of tickets.
// Returns nil for not-found; errors only for infrastructure issues.
type TicketRepository interface {
	Get(ctx context.Context, id int) (*Ticket, error)
	GetByCode(ctx context.Context, code string) (*Ticket, error)
	Add(ctx context.Context, ticket *Ticket) error
	Update(ctx context.Context, ticket *Ticket) error
	FindByPurchaseID(ctx context.Context, purchaseID int) ([]*Ticket, error)
	FindByEventID(ctx context.Context, eventID int) ([]*Ticket, error)
}

// PurchaseRepository provides access to the collection of purchases.
// Returns nil for not-found; errors only for infrastructure issues.
type PurchaseRepository interface {
	Get(ctx context.Context, id int) (*Purchase, error)
	Add(ctx context.Context, purchase *Purchase) error
}
