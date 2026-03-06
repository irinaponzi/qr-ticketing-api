package validator

import "context"

// TicketInfo represents the minimal ticket data returned by the ticket service fallback.
type TicketInfo struct {
	Code    string
	EventID int
	Status  string
}

// TicketServiceClient provides a live fallback to the ticket service
// when a ticket is not found in the validator's local database.
type TicketServiceClient interface {
	GetTicketByCode(ctx context.Context, code string) (*TicketInfo, error)
}
