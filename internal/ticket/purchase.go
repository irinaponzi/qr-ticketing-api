package ticket

import (
	"errors"
	"time"
)

// Purchase represents a completed transaction for one or more tickets.
type Purchase struct {
	id         int
	attributes purchaseAttributes
}

type purchaseAttributes struct {
	buyerEmail string
	eventID    int
	quantity   int
	totalPrice float64
	tickets    []*Ticket
	createdAt  time.Time
}

// NewPurchase creates a new Purchase with the given parameters.
// It validates that the ID and event ID are positive, email is non-empty,
// quantity is positive, and total price is not negative.
//
// Parameters:
//   - id: The unique identifier for the purchase (must be positive).
//   - buyerEmail: The email address of the buyer (must be non-empty).
//   - eventID: The ID of the event being purchased (must be positive).
//   - quantity: The number of tickets to purchase (must be positive).
//   - totalPrice: The total price for all tickets (must be non-negative).
//
// Returns:
//   - *Purchase: A pointer to the newly created Purchase.
//   - error: An error if any invariant is violated; otherwise, nil.
func NewPurchase(id int, buyerEmail string, eventID, quantity int, totalPrice float64) (*Purchase, error) {
	if id < 0 {
		return nil, errors.New("purchase ID cannot be negative")
	}

	if buyerEmail == "" {
		return nil, errors.New("buyer email cannot be empty")
	}

	if eventID <= 0 {
		return nil, errors.New("event ID must be positive")
	}

	if quantity <= 0 {
		return nil, errors.New("quantity must be positive")
	}

	if totalPrice < 0 {
		return nil, errors.New("total price cannot be negative")
	}

	return &Purchase{
		id: id,
		attributes: purchaseAttributes{
			buyerEmail: buyerEmail,
			eventID:    eventID,
			quantity:   quantity,
			totalPrice: totalPrice,
			tickets:    make([]*Ticket, 0, quantity),
			createdAt:  time.Now(),
		},
	}, nil
}

// NewPurchaseFromRepository reconstructs a Purchase from persisted data.
// It bypasses validation since the data is assumed to be consistent.
//
// Parameters:
//   - id: The purchase identifier.
//   - buyerEmail: The buyer's email address.
//   - eventID: The associated event ID.
//   - quantity: The number of tickets in the purchase.
//   - totalPrice: The total price paid.
//   - createdAt: Original creation timestamp.
//   - tickets: The list of tickets associated with this purchase.
//
// Returns:
//   - *Purchase: A pointer to the reconstructed Purchase.
func NewPurchaseFromRepository(id int, buyerEmail string, eventID, quantity int, totalPrice float64, createdAt time.Time, tickets []*Ticket) *Purchase {
	return &Purchase{
		id: id,
		attributes: purchaseAttributes{
			buyerEmail: buyerEmail,
			eventID:    eventID,
			quantity:   quantity,
			totalPrice: totalPrice,
			tickets:    tickets,
			createdAt:  createdAt,
		},
	}
}

// SetID assigns the database-generated ID to the purchase after insertion.
// It can only be called once on a purchase with id = 0 (new entity).
func (p *Purchase) SetID(id int) error {
	if p.id != 0 {
		return errors.New("purchase ID already set")
	}

	if id <= 0 {
		return errors.New("purchase ID must be positive")
	}

	p.id = id

	return nil
}

// ID returns the unique identifier of the purchase.
func (p *Purchase) ID() int { return p.id }

// BuyerEmail returns the email address of the buyer.
func (p *Purchase) BuyerEmail() string { return p.attributes.buyerEmail }

// EventID returns the ID of the event associated with this purchase.
func (p *Purchase) EventID() int { return p.attributes.eventID }

// Quantity returns the number of tickets in this purchase.
func (p *Purchase) Quantity() int { return p.attributes.quantity }

// TotalPrice returns the total price paid for all tickets.
func (p *Purchase) TotalPrice() float64 { return p.attributes.totalPrice }

// Tickets returns the list of tickets associated with this purchase.
func (p *Purchase) Tickets() []*Ticket { return p.attributes.tickets }

// CreatedAt returns the timestamp when the purchase was created.
func (p *Purchase) CreatedAt() time.Time { return p.attributes.createdAt }

// AddTicket assigns a ticket to this purchase.
// The number of tickets cannot exceed the purchase quantity.
//
// Parameters:
//   - t: The ticket to add (must be non-nil).
//
// Returns:
//   - error: An error if the ticket is nil or the purchase already has all
//     tickets assigned; otherwise, nil.
func (p *Purchase) AddTicket(t *Ticket) error {
	if t == nil {
		return errors.New("ticket cannot be nil")
	}

	if len(p.attributes.tickets) >= p.attributes.quantity {
		return errors.New("purchase already has all tickets assigned")
	}

	p.attributes.tickets = append(p.attributes.tickets, t)

	return nil
}

// TicketCodes returns a slice with the UUID codes of all tickets in this purchase.
func (p *Purchase) TicketCodes() []string {
	codes := make([]string, 0, len(p.attributes.tickets))
	for _, t := range p.attributes.tickets {
		codes = append(codes, t.Code())
	}

	return codes
}
