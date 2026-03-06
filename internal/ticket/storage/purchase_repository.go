package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/iponzi/entradasQR/internal/ticket"
)

const (
	queryGetPurchase    = `SELECT id, buyer_email, event_id, quantity, total_price, created_at FROM purchases WHERE id = ?`
	queryInsertPurchase = `INSERT INTO purchases (buyer_email, event_id, quantity, total_price, created_at) VALUES (?, ?, ?, ?, ?)`
)

// MySQLPurchaseRepository implements PurchaseRepository using a MySQL database.
type MySQLPurchaseRepository struct {
	db *sql.DB
}

// NewMySQLPurchaseRepository creates a new purchase repository backed by the given MySQL connection.
func NewMySQLPurchaseRepository(db *sql.DB) *MySQLPurchaseRepository {
	return &MySQLPurchaseRepository{db: db}
}

// Get retrieves a purchase by its ID. Returns nil if not found.
func (r *MySQLPurchaseRepository) Get(ctx context.Context, id int) (*ticket.Purchase, error) {
	row := r.db.QueryRowContext(ctx, queryGetPurchase, id)

	var pr purchaseRow
	err := row.Scan(&pr.ID, &pr.BuyerEmail, &pr.EventID, &pr.Quantity, &pr.TotalPrice, &pr.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("scanning purchase: %w", err)
	}

	return ticket.NewPurchaseFromRepository(pr.ID, pr.BuyerEmail, pr.EventID, pr.Quantity, pr.TotalPrice, pr.CreatedAt, nil), nil
}

// Add persists a new purchase to the database.
// The database assigns the ID via AUTO_INCREMENT and sets it on the entity.
func (r *MySQLPurchaseRepository) Add(ctx context.Context, purchase *ticket.Purchase) error {
	result, err := r.db.ExecContext(ctx, queryInsertPurchase,
		purchase.BuyerEmail(), purchase.EventID(), purchase.Quantity(), purchase.TotalPrice(), purchase.CreatedAt(),
	)
	if err != nil {
		return fmt.Errorf("inserting purchase: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("getting purchase last insert id: %w", err)
	}

	if err := purchase.SetID(int(id)); err != nil {
		return fmt.Errorf("setting purchase id: %w", err)
	}

	return nil
}
