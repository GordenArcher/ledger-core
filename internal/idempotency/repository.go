package idempotency

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// Repository handles DB operations for idempotency records.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new idempotency Repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Find looks up a record by key.
// Returns gorm.ErrRecordNotFound if the key has never been seen,
// or if the record has expired.
func (r *Repository) Find(key string) (*Record, error) {
	var record Record
	err := r.db.
		Where("key = ? AND expires_at > ?", key, time.Now()).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// Save persists a new idempotency record.
func (r *Repository) Save(record *Record) error {
	return r.db.Create(record).Error
}

// DeleteExpired removes all records past their expiry time.
// Called periodically by a Celery-style cleanup job in Phase 7.
func (r *Repository) DeleteExpired() error {
	return r.db.
		Where("expires_at < ?", time.Now()).
		Delete(&Record{}).Error
}

// IsNotFound returns true if the error is a GORM record-not-found error.
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
