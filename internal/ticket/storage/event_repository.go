package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/iponzi/entradasQR/internal/ticket"
)

const (
	queryListEvents  = `SELECT id, name, location, date, capacity, ticket_price, sold_count, created_at, updated_at FROM events ORDER BY date ASC`
	queryGetEvent    = `SELECT id, name, location, date, capacity, ticket_price, sold_count, created_at, updated_at FROM events WHERE id = ?`
	queryInsertEvent = `INSERT INTO events (name, location, date, capacity, ticket_price, sold_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	queryUpdateEvent = `UPDATE events SET name = ?, location = ?, date = ?, capacity = ?, ticket_price = ?, sold_count = ?, updated_at = ? WHERE id = ?`
)

// MySQLEventRepository implements EventRepository using a MySQL database.
type MySQLEventRepository struct {
	db *sql.DB
}

// NewMySQLEventRepository creates a new event repository backed by the given MySQL connection.
func NewMySQLEventRepository(db *sql.DB) *MySQLEventRepository {
	return &MySQLEventRepository{db: db}
}

// List retrieves all events ordered by date ascending.
func (r *MySQLEventRepository) List(ctx context.Context) ([]*ticket.Event, error) {
	rows, err := r.db.QueryContext(ctx, queryListEvents)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []*ticket.Event
	for rows.Next() {
		var evt eventRow
		if err := rows.Scan(&evt.ID, &evt.Name, &evt.Location, &evt.Date, &evt.Capacity, &evt.TicketPrice, &evt.SoldCount, &evt.CreatedAt, &evt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		events = append(events, ticket.NewEventFromRepository(evt.ID, evt.Name, evt.Location, evt.Date, evt.Capacity, evt.TicketPrice, evt.SoldCount, evt.CreatedAt, evt.UpdatedAt))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating events: %w", err)
	}

	return events, nil
}

// Get retrieves an event by its ID. Returns nil if not found.
func (r *MySQLEventRepository) Get(ctx context.Context, id int) (*ticket.Event, error) {
	row := r.db.QueryRowContext(ctx, queryGetEvent, id)

	var evt eventRow
	err := row.Scan(&evt.ID, &evt.Name, &evt.Location, &evt.Date, &evt.Capacity, &evt.TicketPrice, &evt.SoldCount, &evt.CreatedAt, &evt.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("scanning event: %w", err)
	}

	return ticket.NewEventFromRepository(evt.ID, evt.Name, evt.Location, evt.Date, evt.Capacity, evt.TicketPrice, evt.SoldCount, evt.CreatedAt, evt.UpdatedAt), nil
}

// Add persists a new event to the database.
func (r *MySQLEventRepository) Add(ctx context.Context, event *ticket.Event) error {
	result, err := r.db.ExecContext(ctx, queryInsertEvent,
		event.Name(), event.Location(), event.Date(), event.Capacity(), event.TicketPrice(), event.SoldCount(), event.CreatedAt(), event.UpdatedAt(),
	)
	if err != nil {
		return fmt.Errorf("inserting event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting last insert id: %w", err)
	}

	event.SetID(int(id))

	return nil
}

// Update persists changes to an existing event.
func (r *MySQLEventRepository) Update(ctx context.Context, event *ticket.Event) error {
	_, err := r.db.ExecContext(ctx, queryUpdateEvent,
		event.Name(), event.Location(), event.Date(), event.Capacity(), event.TicketPrice(), event.SoldCount(), event.UpdatedAt(), event.ID(),
	)
	if err != nil {
		return fmt.Errorf("updating event: %w", err)
	}

	return nil
}
