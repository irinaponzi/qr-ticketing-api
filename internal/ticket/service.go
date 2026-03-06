package ticket

import (
	"context"
	"fmt"
)

// PurchaseInput represents the data needed to create a purchase.
type PurchaseInput struct {
	BuyerEmail string
	EventID    int
	Quantity   int
}

// PurchaseResult represents the result of a successful purchase.
type PurchaseResult struct {
	PurchaseID int
	Tickets    []*Ticket
	Event      *Event
	TotalPrice float64
}

// TicketService orchestrates the ticket purchasing and cancellation flows.
type TicketService struct {
	eventRepo      EventRepository
	ticketRepo     TicketRepository
	purchaseRepo   PurchaseRepository
	eventPublisher TicketEventPublisher
}

// NewTicketService creates a new TicketService with the given dependencies.
//
// Parameters:
//   - eventRepo: Repository for persisting and retrieving events.
//   - ticketRepo: Repository for persisting and retrieving tickets.
//   - purchaseRepo: Repository for persisting and retrieving purchases.
//   - eventPublisher: Publisher for ticket lifecycle events via message broker.
//
// Returns:
//   - *TicketService: A pointer to the newly created service.
func NewTicketService(
	eventRepo EventRepository,
	ticketRepo TicketRepository,
	purchaseRepo PurchaseRepository,
	eventPublisher TicketEventPublisher,
) *TicketService {
	return &TicketService{
		eventRepo:      eventRepo,
		ticketRepo:     ticketRepo,
		purchaseRepo:   purchaseRepo,
		eventPublisher: eventPublisher,
	}
}

// Purchase orchestrates the ticket purchasing flow: validates event availability,
// reserves tickets, generates unique codes, persists all entities, and publishes
// creation events to the message broker.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - input: The purchase parameters (buyer email, event ID, quantity).
//
// Returns:
//   - *PurchaseResult: The result containing purchase ID, tickets, and event info.
//   - error: ErrNotFound if the event doesn't exist, ErrBusinessRule if not enough
//     tickets, or a wrapped infrastructure error.
func (s *TicketService) Purchase(ctx context.Context, input PurchaseInput) (*PurchaseResult, error) {
	event, err := s.eventRepo.Get(ctx, input.EventID)
	if err != nil {
		return nil, fmt.Errorf("getting event: %w", err)
	}

	if event == nil {
		return nil, fmt.Errorf("%w: event %d", ErrNotFound, input.EventID)
	}

	if err := event.ReserveTickets(input.Quantity); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBusinessRule, err.Error())
	}

	totalPrice := event.TicketPrice() * float64(input.Quantity)

	purchase, err := NewPurchase(0, input.BuyerEmail, input.EventID, input.Quantity, totalPrice)
	if err != nil {
		return nil, fmt.Errorf("creating purchase: %w", err)
	}

	if err := s.purchaseRepo.Add(ctx, purchase); err != nil {
		return nil, fmt.Errorf("persisting purchase: %w", err)
	}

	for i := 0; i < input.Quantity; i++ {
		t, err := NewTicket(0, input.EventID, purchase.ID())
		if err != nil {
			return nil, fmt.Errorf("creating ticket: %w", err)
		}

		if err := s.ticketRepo.Add(ctx, t); err != nil {
			return nil, fmt.Errorf("persisting ticket: %w", err)
		}

		if err := purchase.AddTicket(t); err != nil {
			return nil, fmt.Errorf("adding ticket to purchase: %w", err)
		}

		if err := s.eventPublisher.PublishTicketCreated(ctx, TicketCreatedEvent{
			TicketID:   t.ID(),
			TicketCode: t.Code(),
			EventID:    input.EventID,
		}); err != nil {
			return nil, fmt.Errorf("publishing ticket created event: %w", err)
		}
	}

	if err := s.eventRepo.Update(ctx, event); err != nil {
		return nil, fmt.Errorf("updating event sold count: %w", err)
	}

	if err := s.eventPublisher.PublishPurchaseCompleted(ctx, PurchaseCompletedEvent{
		PurchaseID:  purchase.ID(),
		BuyerEmail:  input.BuyerEmail,
		EventName:   event.Name(),
		TicketCodes: purchase.TicketCodes(),
	}); err != nil {
		return nil, fmt.Errorf("publishing purchase completed event: %w", err)
	}

	return &PurchaseResult{
		PurchaseID: purchase.ID(),
		Tickets:    purchase.Tickets(),
		Event:      event,
		TotalPrice: purchase.TotalPrice(),
	}, nil
}

// CancelTicket cancels a ticket by transitioning it to cancelled status,
// persisting the change, and publishing a cancellation event to the message broker.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - ticketID: The ID of the ticket to cancel.
//
// Returns:
//   - error: ErrNotFound if the ticket doesn't exist, ErrBusinessRule if the
//     ticket cannot be cancelled, or a wrapped infrastructure error.
func (s *TicketService) CancelTicket(ctx context.Context, ticketID int) error {
	t, err := s.ticketRepo.Get(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("getting ticket: %w", err)
	}

	if t == nil {
		return fmt.Errorf("%w: ticket %d", ErrNotFound, ticketID)
	}

	if err := t.Cancel(); err != nil {
		return fmt.Errorf("%w: %s", ErrBusinessRule, err.Error())
	}

	if err := s.ticketRepo.Update(ctx, t); err != nil {
		return fmt.Errorf("updating ticket: %w", err)
	}

	if err := s.eventPublisher.PublishTicketCancelled(ctx, TicketCancelledEvent{
		TicketID:   t.ID(),
		TicketCode: t.Code(),
		EventID:    t.EventID(),
	}); err != nil {
		return fmt.Errorf("publishing ticket cancelled event: %w", err)
	}

	return nil
}

// SyncTicketUsed handles ticket.used events from the Validator Service.
// It marks the ticket as used in the Ticket Service database for bidirectional
// reconciliation. The operation is idempotent: if the ticket is already used,
// it is a no-op.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - code: The UUID code of the ticket that was validated.
//
// Returns:
//   - error: A wrapped infrastructure error if the operation fails.
func (s *TicketService) SyncTicketUsed(ctx context.Context, code string) error {
	t, err := s.ticketRepo.GetByCode(ctx, code)
	if err != nil {
		return fmt.Errorf("getting ticket by code: %w", err)
	}

	if t == nil {
		return nil
	}

	if t.Status() == TicketStatusUsed {
		return nil
	}

	if err := t.MarkAsUsed(); err != nil {
		return fmt.Errorf("marking ticket as used: %w", err)
	}

	if err := s.ticketRepo.Update(ctx, t); err != nil {
		return fmt.Errorf("updating ticket: %w", err)
	}

	return nil
}

// GetTicketByCode retrieves a ticket by its unique UUID code.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - code: The UUID code of the ticket.
//
// Returns:
//   - *Ticket: The matching ticket.
//   - error: ErrNotFound if no ticket matches the code, or a wrapped
//     infrastructure error.
func (s *TicketService) GetTicketByCode(ctx context.Context, code string) (*Ticket, error) {
	t, err := s.ticketRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("getting ticket by code: %w", err)
	}

	if t == nil {
		return nil, fmt.Errorf("%w: ticket with code %s", ErrNotFound, code)
	}

	return t, nil
}
