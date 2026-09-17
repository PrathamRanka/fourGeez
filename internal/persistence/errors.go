package persistence

import "errors"

var (
	ErrNotFound                  = errors.New("record not found")
	ErrAlreadyExists             = errors.New("record already exists")
	ErrConditionFailed           = errors.New("conditional write failed")
	ErrPaymentIdentifierConflict = errors.New("payment identifier already exists")
)
