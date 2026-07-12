package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	appmiddleware "github.com/iponzi/entradasQR/internal/platform/middleware"
	"github.com/iponzi/entradasQR/internal/platform/metrics"
	"github.com/iponzi/entradasQR/internal/validator"
)

// ValidatorHandler handles HTTP requests for QR ticket validation at the venue.
type ValidatorHandler struct {
	service     *validator.ValidatorService
	tokenSigner validator.TokenSigner
	logger      *slog.Logger
}

// NewValidatorHandler creates a new ValidatorHandler with the given dependencies.
//
// Parameters:
//   - service: The validator domain service for ticket validation logic.
//   - logger: Structured logger for observability.
//
// Returns:
//   - *ValidatorHandler: A pointer to the newly created handler.
func NewValidatorHandler(service *validator.ValidatorService, tokenSigner validator.TokenSigner, logger *slog.Logger) *ValidatorHandler {
	return &ValidatorHandler{
		service:     service,
		tokenSigner: tokenSigner,
		logger:      logger,
	}
}

// Routes returns a chi.Router with the validation endpoint protected for admins.
//
// Role mapping:
//   - POST /validate → admin only
func (h *ValidatorHandler) Routes(validator appmiddleware.TokenValidator) chi.Router {
	r := chi.NewRouter()

	r.With(appmiddleware.RequireRole(validator, appmiddleware.RoleAdmin)).Post("/validate", h.ValidateTicket)

	return r
}

// --- Request / Response DTOs ---

type validateRequest struct {
	Token string `json:"token"`
}

type validateResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// --- Handlers ---

// ValidateTicket handles POST /validate.
// It reads the ticket code from the request body, delegates validation to the
// ValidatorService (dual validation: local DB + HTTP fallback), and returns
// the result as JSON with status 200.
func (h *ValidatorHandler) ValidateTicket(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	if req.Token == "" {
		respondJSON(w, http.StatusBadRequest, errorResponse{Error: "token is required"})
		return
	}

	code, valid := h.tokenSigner.Verify(req.Token)
	if !valid {
		metrics.TicketsValidatedTotal.WithLabelValues("invalid").Inc()
		respondJSON(w, http.StatusOK, validateResponse{Valid: false, Message: "invalid or tampered QR token"})

		return
	}

	result, err := h.service.ValidateTicket(r.Context(), code)
	if err != nil {
		if errors.Is(err, validator.ErrBusinessRule) {
			respondJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
			return
		}

		h.logger.ErrorContext(r.Context(), "validation error", "error", err)
		respondJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})

		return
	}

	if result.Valid {
		metrics.TicketsValidatedTotal.WithLabelValues("valid").Inc()
	} else {
		metrics.TicketsValidatedTotal.WithLabelValues("invalid").Inc()
	}

	h.logger.InfoContext(r.Context(), "ticket validation", "token", req.Token, "valid", result.Valid, "message", result.Message)

	respondJSON(w, http.StatusOK, validateResponse{
		Valid:   result.Valid,
		Message: result.Message,
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
