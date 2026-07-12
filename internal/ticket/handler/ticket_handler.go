package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	appmiddleware "github.com/iponzi/entradasQR/internal/platform/middleware"
	"github.com/iponzi/entradasQR/internal/platform/metrics"
	"github.com/iponzi/entradasQR/internal/ticket"
)

// TicketHandler handles HTTP requests for ticket-related operations
// including event creation, ticket purchasing, retrieval, and cancellation.
type TicketHandler struct {
	service   *ticket.TicketService
	eventRepo ticket.EventRepository
	logger    *slog.Logger
}

// NewTicketHandler creates a new TicketHandler with the given dependencies.
//
// Parameters:
//   - service: The ticket domain service for business operations.
//   - eventRepo: Repository for event persistence (used for direct event creation).
//   - logger: Structured logger for observability.
//
// Returns:
//   - *TicketHandler: A pointer to the newly created handler.
func NewTicketHandler(
	service *ticket.TicketService,
	eventRepo ticket.EventRepository,
	logger *slog.Logger,
) *TicketHandler {
	return &TicketHandler{
		service:   service,
		eventRepo: eventRepo,
		logger:    logger,
	}
}

// Routes returns a chi.Router with ticket routes.
//
// Public (no auth):
//   - GET  /events        → list all events
//   - GET  /events/{id}   → get single event
//
// Role-protected:
//   - POST /events              → admin only
//   - POST /purchases           → user only
//   - POST /tickets/lookup      → any authenticated caller
//   - POST /tickets/cancel      → admin only
func (h *TicketHandler) Routes(validator appmiddleware.TokenValidator) chi.Router {
	r := chi.NewRouter()

	r.Get("/events", h.ListEvents)
	r.Get("/events/{id}", h.GetEvent)
	r.With(appmiddleware.RequireRole(validator, appmiddleware.RoleAdmin)).Post("/events", h.CreateEvent)
	r.With(appmiddleware.RequireRole(validator, appmiddleware.RoleUser)).Post("/purchases", h.CreatePurchase)
	r.With(appmiddleware.RequireAuth(validator)).Post("/tickets/lookup", h.LookupTicket)
	r.With(appmiddleware.RequireRole(validator, appmiddleware.RoleAdmin)).Post("/tickets/cancel", h.CancelTicket)

	return r
}

// --- Request / Response DTOs ---

type eventResponse struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	Location         string  `json:"location"`
	Date             string  `json:"date"`
	Capacity         int     `json:"capacity"`
	AvailableTickets int     `json:"available_tickets"`
	TicketPrice      float64 `json:"ticket_price"`
}

type createEventRequest struct {
	Name        string  `json:"name"`
	Location    string  `json:"location"`
	Date        string  `json:"date"`
	Capacity    int     `json:"capacity"`
	TicketPrice float64 `json:"ticket_price"`
}

type createEventResponse struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Location    string  `json:"location"`
	Date        string  `json:"date"`
	Capacity    int     `json:"capacity"`
	TicketPrice float64 `json:"ticket_price"`
}

type createPurchaseRequest struct {
	BuyerEmail string `json:"buyer_email"`
	EventID    int    `json:"event_id"`
	Quantity   int    `json:"quantity"`
}

type createPurchaseResponse struct {
	PurchaseID int              `json:"purchase_id"`
	EventName  string           `json:"event_name"`
	Tickets    []ticketResponse `json:"tickets"`
	TotalPrice float64          `json:"total_price"`
}

type ticketResponse struct {
	ID     int    `json:"id"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

type lookupTicketRequest struct {
	Code string `json:"code"`
}

type cancelTicketRequest struct {
	Code string `json:"code"`
}

type cancelTicketResponse struct {
	Message string `json:"message"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// --- Handlers ---

// ListEvents handles GET /events.
// Returns all events ordered by date, no authentication required.
func (h *TicketHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.service.ListEvents(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to list events", "error", err)
		respondJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	resp := make([]eventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, toEventResponse(e))
	}

	respondJSON(w, http.StatusOK, resp)
}

// GetEvent handles GET /events/{id}.
// Returns a single event by ID, no authentication required.
func (h *TicketHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid event id"})
		return
	}

	event, err := h.service.GetEvent(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, toEventResponse(event))
}

// CreateEvent handles POST /events.
// It parses the request body, validates the event data, persists the event,
// and returns the created event as JSON with status 201.
func (h *TicketHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	parsedDate, err := parseDate(req.Date)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid date format, use RFC3339"})
		return
	}

	event, err := ticket.NewEvent(0, req.Name, req.Location, parsedDate, req.Capacity, req.TicketPrice)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if err := h.eventRepo.Add(r.Context(), event); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to create event", "error", err)
		respondJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	metrics.EventsCreatedTotal.Inc()
	h.logger.InfoContext(r.Context(), "event created", "name", event.Name(), "capacity", event.Capacity())

	respondJSON(w, http.StatusCreated, createEventResponse{
		ID:          event.ID(),
		Name:        event.Name(),
		Location:    event.Location(),
		Date:        event.Date().Format("2006-01-02T15:04:05Z07:00"),
		Capacity:    event.Capacity(),
		TicketPrice: event.TicketPrice(),
	})
}

// CreatePurchase handles POST /purchases.
// It delegates to the TicketService to orchestrate the purchase flow,
// then asynchronously generates QR codes and sends a confirmation email.
func (h *TicketHandler) CreatePurchase(w http.ResponseWriter, r *http.Request) {
	var req createPurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	result, err := h.service.Purchase(r.Context(), ticket.PurchaseInput{
		BuyerEmail: req.BuyerEmail,
		EventID:    req.EventID,
		Quantity:   req.Quantity,
	})
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	tickets := make([]ticketResponse, 0, len(result.Tickets))
	for _, t := range result.Tickets {
		tickets = append(tickets, ticketResponse{
			ID:     t.ID(),
			Code:   t.Code(),
			Status: string(t.Status()),
		})
	}

	metrics.TicketsPurchasedTotal.Add(float64(len(result.Tickets)))
	h.logger.InfoContext(r.Context(), "purchase completed", "purchase_id", result.PurchaseID, "quantity", len(result.Tickets))

	respondJSON(w, http.StatusCreated, createPurchaseResponse{
		PurchaseID: result.PurchaseID,
		EventName:  result.Event.Name(),
		Tickets:    tickets,
		TotalPrice: result.TotalPrice,
	})
}

// LookupTicket handles POST /tickets/lookup.
// It retrieves a ticket by its UUID code from the request body and returns it as JSON.
func (h *TicketHandler) LookupTicket(w http.ResponseWriter, r *http.Request) {
	var req lookupTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	if req.Code == "" {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: "code is required"})
		return
	}

	t, err := h.service.GetTicketByCode(r.Context(), req.Code)
	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, ticketResponse{
		ID:     t.ID(),
		Code:   t.Code(),
		Status: string(t.Status()),
	})
}

// CancelTicket handles POST /tickets/cancel.
// It cancels the ticket by code from the request body and returns 200 with a confirmation message.
func (h *TicketHandler) CancelTicket(w http.ResponseWriter, r *http.Request) {
	var req cancelTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	if req.Code == "" {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: "code is required"})
		return
	}

	if err := h.service.CancelTicket(r.Context(), req.Code); err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, cancelTicketResponse{Message: "ticket cancelled"})
}

// --- Helpers ---

func (h *TicketHandler) handleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, ticket.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
		return
	}

	if errors.Is(err, ticket.ErrBusinessRule) {
		respondJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
		return
	}

	h.logger.ErrorContext(r.Context(), "unexpected error", "error", err)
	respondJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
}

func toEventResponse(e *ticket.Event) eventResponse {
	return eventResponse{
		ID:               e.ID(),
		Name:             e.Name(),
		Location:         e.Location(),
		Date:             e.Date().Format(time.RFC3339),
		Capacity:         e.Capacity(),
		AvailableTickets: e.AvailableTickets(),
		TicketPrice:      e.TicketPrice(),
	}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func parseDate(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
