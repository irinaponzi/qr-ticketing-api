package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/iponzi/entradasQR/internal/validator"
)

// TicketServiceHTTPClient implements the live fallback to the ticket service via HTTP.
type TicketServiceHTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewTicketServiceHTTPClient creates a new HTTP client for the ticket service fallback.
// It uses a 3-second timeout to avoid blocking the validation flow.
//
// Parameters:
//   - baseURL: The base URL of the ticket service (e.g., "http://localhost:8080").
//
// Returns:
//   - *TicketServiceHTTPClient: A pointer to the newly created client.
func NewTicketServiceHTTPClient(baseURL string) *TicketServiceHTTPClient {
	return &TicketServiceHTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

type ticketAPIResponse struct {
	ID     int    `json:"id"`
	Code   string `json:"code"`
	Status string `json:"status"`
}

// GetTicketByCode queries the ticket service via HTTP POST /tickets/lookup.
// Returns nil if the ticket is not found (404). Returns an error for any
// other non-200 response or network failure.
//
// Parameters:
//   - ctx: The request context for cancellation and tracing.
//   - code: The UUID code of the ticket to look up.
//
// Returns:
//   - *validator.TicketInfo: The ticket information, or nil if not found.
//   - error: A wrapped error if the request fails; otherwise, nil.
func (c *TicketServiceHTTPClient) GetTicketByCode(ctx context.Context, code string) (*validator.TicketInfo, error) {
	url := fmt.Sprintf("%s/tickets/lookup", c.baseURL)

	body, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ticket service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var apiResp ticketAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &validator.TicketInfo{
		Code:    apiResp.Code,
		EventID: 0,
		Status:  apiResp.Status,
	}, nil
}
