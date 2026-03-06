package ticket

import "errors"

var (
	// ErrNotFound is returned when a requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrBusinessRule is returned when a domain invariant or business rule is violated.
	ErrBusinessRule = errors.New("business rule violation")
)
