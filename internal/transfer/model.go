package transfer

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TransferStatus represents the outcome of a transfer attempt.
type TransferStatus string

const (
	// TransferStatusCompleted means both debit and credit were applied successfully.
	TransferStatusCompleted TransferStatus = "completed"

	// TransferStatusFailed means the transfer was attempted but rolled back.
	TransferStatusFailed TransferStatus = "failed"
)

// Transfer records a movement of funds between two accounts.
//
// Design decisions:
//   - Both FromAccountID and ToAccountID are stored so we can query
//     all transfers involving a given account in either direction.
//   - Amount is in minor units (pesewas/cents), consistent with Account.Balance.
//   - Reference is a human-readable identifier (e.g. "school fees", "rent").
//   - IdempotencyKey ensures the same transfer request cannot be processed twice
//     even if the client retries. Added in Phase 6.
type Transfer struct {
	// UUID primary key
	ID string `gorm:"type:uuid;primaryKey" json:"id"`

	// The account being debited (sender)
	FromAccountID string `gorm:"type:uuid;not null;index" json:"from_account_id"`

	// The account being credited (receiver)
	ToAccountID string `gorm:"type:uuid;not null;index" json:"to_account_id"`

	// Amount in minor units. Always positive.
	Amount int64 `gorm:"not null" json:"amount"`

	// Currency of the transfer — must match both accounts' currency
	Currency string `gorm:"type:varchar(10);not null" json:"currency"`

	// Optional human-readable description of the transfer purpose
	Reference string `gorm:"type:varchar(255)" json:"reference"`

	// Outcome of the transfer
	Status TransferStatus `gorm:"type:varchar(20);not null;default:'completed'" json:"status"`

	// IdempotencyKey prevents duplicate transfers on client retries.
	// Unique index enforced at the DB level.
	IdempotencyKey string `gorm:"type:varchar(255);uniqueIndex" json:"idempotency_key,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate generates a UUID primary key before inserting a new transfer.
func (t *Transfer) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}
