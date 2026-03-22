package ledger

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EntryType represents the direction of a ledger entry.
// In double-entry bookkeeping, every transaction has at least one debit
// and one credit. The sum of all debits must always equal the sum of all credits.
type EntryType string

const (
	// EntryTypeDebit means money is leaving the account (withdrawal, transfer out)
	EntryTypeDebit EntryType = "debit"

	// EntryTypeCredit means money is entering the account (deposit, transfer in)
	EntryTypeCredit EntryType = "credit"
)

// EntrySource identifies what operation produced this ledger entry.
// Useful for filtering history by operation type.
type EntrySource string

const (
	EntrySourceDeposit  EntrySource = "deposit"
	EntrySourceWithdraw EntrySource = "withdraw"
	EntrySourceTransfer EntrySource = "transfer"
)

// Entry is a single line in the ledger, one side of a financial event.
//
// Design decisions:
//   - Every deposit, withdrawal, and transfer writes entries here.
//   - TransferID is nullable, only set when the source is a transfer.
//   - BalanceAfter captures the account balance at the moment of the entry.
//     This makes it easy to reconstruct a statement without replaying history.
//   - Amount is always positive. Direction is captured by EntryType.
type Entry struct {
	// UUID primary key
	ID string `gorm:"type:uuid;primaryKey" json:"id"`

	// The account this entry belongs to
	AccountID string `gorm:"type:uuid;not null;index" json:"account_id"`

	// The type of entry, debit (money out) or credit (money in)
	Type EntryType `gorm:"type:varchar(10);not null" json:"type"`

	// The operation that triggered this entry
	Source EntrySource `gorm:"type:varchar(20);not null" json:"source"`

	// Amount in minor units (pesewas/cents). Always positive.
	Amount int64 `gorm:"not null" json:"amount"`

	// The account balance immediately after this entry was applied.
	// Useful for generating account statements.
	BalanceAfter int64 `gorm:"not null" json:"balance_after"`

	// TransferID links this entry back to its parent transfer.
	// NULL for deposits and withdrawals.
	TransferID *string `gorm:"type:uuid;index" json:"transfer_id,omitempty"`

	// Human-readable note about this entry
	Description string `gorm:"type:varchar(255)" json:"description"`

	// Timestamp
	CreatedAt time.Time `json:"created_at"`
}

// BeforeCreate generates a UUID primary key before inserting a new entry.
func (e *Entry) BeforeCreate(tx *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return nil
}
