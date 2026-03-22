package transfer

import (
	"errors"

	"gorm.io/gorm"
)

// Repository handles all database operations for transfers.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new transfer Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new transfer record within the given transaction.
// Always called inside a transaction managed by the service layer.
func (r *Repository) Create(tx *gorm.DB, transfer *Transfer) error {
	return tx.Create(transfer).Error
}

// FindByID retrieves a transfer by its UUID.
func (r *Repository) FindByID(id string) (*Transfer, error) {
	var transfer Transfer
	err := r.db.Where("id = ?", id).First(&transfer).Error
	if err != nil {
		return nil, err
	}
	return &transfer, nil
}

// FindByIdempotencyKey looks up a transfer by its idempotency key.
// Returns gorm.ErrRecordNotFound if no match exists, meaning this is a new request.
func (r *Repository) FindByIdempotencyKey(key string) (*Transfer, error) {
	var transfer Transfer
	err := r.db.Where("idempotency_key = ?", key).First(&transfer).Error
	if err != nil {
		return nil, err
	}
	return &transfer, nil
}

// FindByAccount returns all transfers where the given account is either
// the sender or the receiver, ordered by most recent first.
func (r *Repository) FindByAccount(accountID string) ([]Transfer, error) {
	var transfers []Transfer
	err := r.db.
		Where("from_account_id = ? OR to_account_id = ?", accountID, accountID).
		Order("created_at DESC").
		Find(&transfers).Error
	if err != nil {
		return nil, err
	}
	return transfers, nil
}

// IsNotFound returns true if the error is a GORM record-not-found error.
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
