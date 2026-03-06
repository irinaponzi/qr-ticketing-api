package validator

import (
	"context"
	"fmt"
	"log/slog"
)

// ValidationResult represents the outcome of a ticket validation.
type ValidationResult struct {
	Valid   bool
	Message string
}

// ValidatorService handles QR code validation using the dual validation pattern:
// 1. Check local DB first (fast path)
// 2. If not found locally, fallback to ticket service (live query)
type ValidatorService struct {
	validTicketRepo     ValidTicketRepository
	ticketServiceClient TicketServiceClient
	eventPublisher      ValidatorEventPublisher
	logger              *slog.Logger
}

// NewValidatorService creates a new ValidatorService with the given dependencies.
//
// Parameters:
//   - validTicketRepo: Repository for persisting and retrieving valid tickets.
//   - ticketServiceClient: HTTP client for fallback queries to the ticket service.
//   - eventPublisher: Publisher for validation events (ticket.used) for reconciliation.
//   - logger: Structured logger for observability.
//
// Returns:
//   - *ValidatorService: A pointer to the newly created service.
func NewValidatorService(
	validTicketRepo ValidTicketRepository,
	ticketServiceClient TicketServiceClient,
	eventPublisher ValidatorEventPublisher,
	logger *slog.Logger,
) *ValidatorService {
	return &ValidatorService{
		validTicketRepo:     validTicketRepo,
		ticketServiceClient: ticketServiceClient,
		eventPublisher:      eventPublisher,
		logger:              logger,
	}
}

// ValidateTicket validates a ticket using the dual validation pattern:
// first checks the local DB (fast path), and if not found, falls back to
// querying the ticket service via HTTP (live path).
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - code: The UUID code of the ticket to validate.
//
// Returns:
//   - *ValidationResult: The outcome indicating if the ticket is valid and a message.
//   - error: A wrapped infrastructure error if the local DB check fails.
func (s *ValidatorService) ValidateTicket(ctx context.Context, code string) (*ValidationResult, error) {
	// Fast path: check local DB.
	vt, err := s.validTicketRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("checking local db: %w", err)
	}

	if vt != nil {
		return s.validateLocal(ctx, vt)
	}

	// Fallback: query ticket service live.
	s.logger.InfoContext(ctx, "ticket not found locally, falling back to ticket service", "code", code)

	return s.validateFallback(ctx, code)
}

func (s *ValidatorService) validateLocal(ctx context.Context, vt *ValidTicket) (*ValidationResult, error) {
	if !vt.IsActive() {
		return &ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("ticket is %s", string(vt.Status())),
		}, nil
	}

	if err := vt.MarkAsUsed(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBusinessRule, err.Error())
	}

	if err := s.validTicketRepo.Update(ctx, vt); err != nil {
		return nil, fmt.Errorf("updating ticket status: %w", err)
	}

	if err := s.eventPublisher.PublishTicketUsed(ctx, TicketUsedEvent{
		TicketCode: vt.Code(),
		EventID:    vt.EventID(),
	}); err != nil {
		s.logger.ErrorContext(ctx, "failed to publish ticket.used event", "code", vt.Code(), "error", err)
	}

	return &ValidationResult{
		Valid:   true,
		Message: "ticket validated successfully",
	}, nil
}

func (s *ValidatorService) validateFallback(ctx context.Context, code string) (*ValidationResult, error) {
	info, err := s.ticketServiceClient.GetTicketByCode(ctx, code)
	if err != nil {
		s.logger.ErrorContext(ctx, "ticket service fallback failed", "error", err)

		return &ValidationResult{
			Valid:   false,
			Message: "unable to validate ticket at this time",
		}, nil
	}

	if info == nil {
		return &ValidationResult{
			Valid:   false,
			Message: "ticket not found",
		}, nil
	}

	if info.Status != "emitted" {
		return &ValidationResult{
			Valid:   false,
			Message: fmt.Sprintf("ticket is %s", info.Status),
		}, nil
	}

	// Sync to local DB for future validations (idempotent upsert).
	vt, err := NewValidTicket(0, info.Code, info.EventID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to create local ticket record", "error", err)
	} else {
		if err := vt.MarkAsUsed(); err == nil {
			if addErr := s.validTicketRepo.Add(ctx, vt); addErr != nil {
				s.logger.ErrorContext(ctx, "failed to sync ticket locally", "error", addErr)
			}
		}
	}

	if err := s.eventPublisher.PublishTicketUsed(ctx, TicketUsedEvent{
		TicketCode: info.Code,
		EventID:    info.EventID,
	}); err != nil {
		s.logger.ErrorContext(ctx, "failed to publish ticket.used event", "code", info.Code, "error", err)
	}

	return &ValidationResult{
		Valid:   true,
		Message: "ticket validated via live check",
	}, nil
}

// SyncTicketCreated handles ticket.created events from the message broker.
// It creates a local copy of the ticket for future validations.
// The operation is idempotent: if the ticket already exists locally, it is a no-op.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - code: The UUID code of the created ticket.
//   - eventID: The ID of the event the ticket belongs to.
//
// Returns:
//   - error: A wrapped infrastructure error if the operation fails.
func (s *ValidatorService) SyncTicketCreated(ctx context.Context, code string, eventID int) error {
	existing, err := s.validTicketRepo.GetByCode(ctx, code)
	if err != nil {
		return fmt.Errorf("checking existing ticket: %w", err)
	}

	// Idempotent: if already exists, skip.
	if existing != nil {
		return nil
	}

	vt, err := NewValidTicket(0, code, eventID)
	if err != nil {
		return fmt.Errorf("creating valid ticket: %w", err)
	}

	if err := s.validTicketRepo.Add(ctx, vt); err != nil {
		return fmt.Errorf("persisting valid ticket: %w", err)
	}

	return nil
}

// SyncTicketCancelled handles ticket.cancelled events from the message broker.
// It marks the local ticket copy as cancelled. If the ticket doesn't exist
// locally, the operation is a no-op.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - code: The UUID code of the cancelled ticket.
//
// Returns:
//   - error: ErrBusinessRule if the ticket cannot be cancelled, or a wrapped
//     infrastructure error.
func (s *ValidatorService) SyncTicketCancelled(ctx context.Context, code string) error {
	vt, err := s.validTicketRepo.GetByCode(ctx, code)
	if err != nil {
		return fmt.Errorf("getting ticket: %w", err)
	}

	if vt == nil {
		return nil
	}

	if err := vt.MarkAsCancelled(); err != nil {
		return fmt.Errorf("%w: %s", ErrBusinessRule, err.Error())
	}

	if err := s.validTicketRepo.Update(ctx, vt); err != nil {
		return fmt.Errorf("updating ticket: %w", err)
	}

	return nil
}
