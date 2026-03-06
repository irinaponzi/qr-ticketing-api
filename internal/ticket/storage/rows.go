package storage

import "time"

type eventRow struct {
	ID          int
	Name        string
	Location    string
	Date        time.Time
	Capacity    int
	TicketPrice float64
	SoldCount   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ticketRow struct {
	ID         int
	Code       string
	EventID    int
	PurchaseID int
	Status     string
	UsedAt     *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type purchaseRow struct {
	ID         int
	BuyerEmail string
	EventID    int
	Quantity   int
	TotalPrice float64
	CreatedAt  time.Time
}
