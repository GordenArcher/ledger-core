package account

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AccountStatus represents the current state of an account.
// Only active accounts can send or receive funds.
type AccountStatus string

const (
	AccountStatusActive   AccountStatus = "active"
	AccountStatusInactive AccountStatus = "inactive"
	AccountStatusFrozen   AccountStatus = "frozen"
)

// AccountCurrency represents supported currencies.
// Using explicit constants prevents arbitrary strings from being stored.
type AccountCurrency string

const (
	CurrencyGHS AccountCurrency = "GHS" // Ghana Cedi
	CurrencyUSD AccountCurrency = "USD"
	CurrencyEUR AccountCurrency = "EUR"
)

// Account represents a ledger account that can hold a balance and participate in transactions.
//
// Design decisions:
//   - Balance is stored as int64 (pesewas/cents) to avoid floating-point precision issues.
//     GHS 10.50 is stored as 1050. This is standard practice in financial systems.
//   - UUIDs are used as primary keys instead of auto-increment integers to prevent
//     enumeration attacks and allow distributed ID generation.
//   - UpdatedAt is tracked by GORM automatically — useful for audit trails.
type Account struct {
	// ID is a UUID primary key generated before insert (see BeforeCreate hook below)
	ID string `gorm:"type:uuid;primaryKey" json:"id"`

	// OwnerName is the display name of the account holder
	OwnerName string `gorm:"type:varchar(255);not null" json:"owner_name"`

	// Email uniquely identifies the account owner
	Email string `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`

	// Balance is stored in the smallest currency unit (pesewas for GHS, cents for USD).
	// Always read with SELECT FOR UPDATE when modifying to prevent race conditions.
	Balance int64 `gorm:"not null;default:0" json:"balance"`

	// Currency is the ISO 4217 currency code for this account
	Currency AccountCurrency `gorm:"type:varchar(10);not null;default:'GHS'" json:"currency"`

	// Status controls whether the account can send or receive funds
	Status AccountStatus `gorm:"type:varchar(20);not null;default:'active'" json:"status"`

	// Timestamps managed automatically by GORM
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate is a GORM hook that runs before inserting a new account.
// It generates a UUID for the primary key if one hasn't been set.
func (a *Account) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// BalanceInMajorUnit returns the balance as a float64 in the major currency unit.
// e.g. 1050 pesewas → 10.50 GHS. Use only for display purposes, never for arithmetic.
func (a *Account) BalanceInMajorUnit() float64 {
	return float64(a.Balance) / 100
}
