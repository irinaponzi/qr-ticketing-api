package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/iponzi/entradasQR/internal/ticket"
)

const (
	queryGetTicketByID         = `SELECT id, code, event_id, purchase_id, status, used_at, created_at, updated_at FROM tickets WHERE id = ?`
	queryGetTicketByCode       = `SELECT id, code, event_id, purchase_id, status, used_at, created_at, updated_at FROM tickets WHERE code = ?`
	queryInsertTicket          = `INSERT INTO tickets (code, event_id, purchase_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	queryUpdateTicket          = `UPDATE tickets SET status = ?, used_at = ?, updated_at = ? WHERE id = ?`
	queryFindTicketsByPurchase = `SELECT id, code, event_id, purchase_id, status, used_at, created_at, updated_at FROM tickets WHERE purchase_id = ?`
	queryFindTicketsByEvent    = `SELECT id, code, event_id, purchase_id, status, used_at, created_at, updated_at FROM tickets WHERE event_id = ?`
)

// MySQLTicketRepository implements TicketRepository using a MySQL database.
type MySQLTicketRepository struct {
	db *sql.DB
}

// NewMySQLTicketRepository creates a new ticket repository backed by the given MySQL connection.
func NewMySQLTicketRepository(db *sql.DB) *MySQLTicketRepository {
	return &MySQLTicketRepository{db: db}
}

// Get retrieves a ticket by its ID. Returns nil if not found.
func (r *MySQLTicketRepository) Get(ctx context.Context, id int) (*ticket.Ticket, error) {
	row := r.db.QueryRowContext(ctx, queryGetTicketByID, id)

	return r.scanTicket(row)
}

// GetByCode retrieves a ticket by its UUID code. Returns nil if not found.
func (r *MySQLTicketRepository) GetByCode(ctx context.Context, code string) (*ticket.Ticket, error) {
	row := r.db.QueryRowContext(ctx, queryGetTicketByCode, code)

	return r.scanTicket(row)
}

// Add persists a new ticket to the database.
// The database assigns the ID via AUTO_INCREMENT and sets it on the entity.
func (r *MySQLTicketRepository) Add(ctx context.Context, t *ticket.Ticket) error {
	result, err := r.db.ExecContext(ctx, queryInsertTicket,
		t.Code(), t.EventID(), t.PurchaseID(), string(t.Status()), t.CreatedAt(), t.UpdatedAt(),
	)
	if err != nil {
		return fmt.Errorf("inserting ticket: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting ticket last insert id: %w", err)
	}

	if err := t.SetID(int(id)); err != nil {
		return fmt.Errorf("setting ticket id: %w", err)
	}

	return nil
}

// Update persists changes to an existing ticket.
func (r *MySQLTicketRepository) Update(ctx context.Context, t *ticket.Ticket) error {
	_, err := r.db.ExecContext(ctx, queryUpdateTicket,
		string(t.Status()), t.UsedAt(), t.UpdatedAt(), t.ID(),
	)
	if err != nil {
		return fmt.Errorf("updating ticket: %w", err)
	}

	return nil
}

// FindByPurchaseID returns all tickets belonging to a specific purchase.
func (r *MySQLTicketRepository) FindByPurchaseID(ctx context.Context, purchaseID int) ([]*ticket.Ticket, error) {
	return r.queryTickets(ctx, queryFindTicketsByPurchase, purchaseID)
}

// FindByEventID returns all tickets belonging to a specific event.
func (r *MySQLTicketRepository) FindByEventID(ctx context.Context, eventID int) ([]*ticket.Ticket, error) {
	return r.queryTickets(ctx, queryFindTicketsByEvent, eventID)
}

func (r *MySQLTicketRepository) scanTicket(row *sql.Row) (*ticket.Ticket, error) {
	var tr ticketRow
	err := row.Scan(&tr.ID, &tr.Code, &tr.EventID, &tr.PurchaseID, &tr.Status, &tr.UsedAt, &tr.CreatedAt, &tr.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("scanning ticket: %w", err)
	}

	return ticket.NewTicketFromRepository(tr.ID, tr.Code, tr.EventID, tr.PurchaseID, ticket.TicketStatus(tr.Status), tr.UsedAt, tr.CreatedAt, tr.UpdatedAt), nil
}

func (r *MySQLTicketRepository) queryTickets(ctx context.Context, query string, args ...interface{}) ([]*ticket.Ticket, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying tickets: %w", err)
	}
	defer rows.Close()

	var tickets []*ticket.Ticket

	for rows.Next() {
		var tr ticketRow
		if err := rows.Scan(&tr.ID, &tr.Code, &tr.EventID, &tr.PurchaseID, &tr.Status, &tr.UsedAt, &tr.CreatedAt, &tr.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning ticket row: %w", err)
		}

		tickets = append(tickets, ticket.NewTicketFromRepository(tr.ID, tr.Code, tr.EventID, tr.PurchaseID, ticket.TicketStatus(tr.Status), tr.UsedAt, tr.CreatedAt, tr.UpdatedAt))
	}

	return tickets, rows.Err()
}
