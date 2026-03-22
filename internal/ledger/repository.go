package ledger

import (
	"gorm.io/gorm"
)

// Repository handles all database operations for ledger entries.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new ledger Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new ledger entry within the given transaction.
// Always called inside a transaction managed by the caller — this ensures
// the entry is only persisted if the balance update also succeeds.
func (r *Repository) Create(tx *gorm.DB, entry *Entry) error {
	return tx.Create(entry).Error
}

// FindByAccount returns all ledger entries for the given account,
// ordered by most recent first.
func (r *Repository) FindByAccount(accountID string) ([]Entry, error) {
	var entries []Entry
	err := r.db.
		Where("account_id = ?", accountID).
		Order("created_at DESC").
		Find(&entries).Error
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// FindByAccountPaginated returns a page of ledger entries for the given account.
// offset is the number of records to skip, limit is the page size.
func (r *Repository) FindByAccountPaginated(accountID string, offset, limit int) ([]Entry, int64, error) {
	var entries []Entry
	var total int64

	// Count total entries for pagination metadata
	if err := r.db.Model(&Entry{}).
		Where("account_id = ?", accountID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch the requested page
	if err := r.db.
		Where("account_id = ?", accountID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

// FindByTransfer returns all ledger entries associated with a specific transfer.
// A transfer always produces exactly two entries (one debit, one credit).
func (r *Repository) FindByTransfer(transferID string) ([]Entry, error) {
	var entries []Entry
	err := r.db.
		Where("transfer_id = ?", transferID).
		Order("created_at ASC").
		Find(&entries).Error
	if err != nil {
		return nil, err
	}
	return entries, nil
}
