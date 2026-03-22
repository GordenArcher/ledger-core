package account

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// Service errors — returned by the service layer and handled in the handler.
// Using typed errors lets the handler make decisions based on error type
// rather than string matching.
var (
	ErrAccountNotFound     = errors.New("account not found")
	ErrEmailAlreadyExists  = errors.New("an account with this email already exists")
	ErrInvalidCurrency     = errors.New("unsupported currency, must be one of: GHS, USD, EUR")
	ErrInvalidAmount       = errors.New("amount must be greater than zero")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrAccountNotActive    = errors.New("account is not active")
)

// LedgerRecorder is an interface the account service uses to write ledger entries.
// Defined here as an interface (not a concrete type) to avoid an import cycle —
// the ledger package would otherwise import account and account would import ledger.
type LedgerRecorder interface {
	RecordDeposit(tx *gorm.DB, accountID string, amount, balanceAfter int64) error
	RecordWithdrawal(tx *gorm.DB, accountID string, amount, balanceAfter int64) error
}

// Service contains the business logic for account operations.
// It sits between the HTTP handler and the repository.
type Service struct {
	repo   *Repository
	ledger LedgerRecorder
}

// NewService creates a new account Service with the given repository.
// ledger is optional, pass nil to skip ledger writes (useful in tests).
func NewService(repo *Repository, ledger LedgerRecorder) *Service {
	return &Service{repo: repo, ledger: ledger}
}

// CreateAccountInput is the validated input for creating a new account.
type CreateAccountInput struct {
	OwnerName string
	Email     string
	Currency  AccountCurrency
}

// CreateAccount validates the input and creates a new account.
// Returns the created account or a descriptive error.
func (s *Service) CreateAccount(input CreateAccountInput) (*Account, error) {
	// Normalize email to lowercase to prevent duplicate accounts
	// from "User@example.com" and "user@example.com"
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	// Validate currency, only allow known values
	if !isValidCurrency(input.Currency) {
		return nil, ErrInvalidCurrency
	}

	// Check for duplicate email before attempting insert
	exists, err := s.repo.ExistsByEmail(input.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	// Build and persist the new account
	// Balance defaults to 0 (set in the model's GORM tag)
	account := &Account{
		OwnerName: strings.TrimSpace(input.OwnerName),
		Email:     input.Email,
		Currency:  input.Currency,
		Status:    AccountStatusActive,
	}

	if err := s.repo.Create(account); err != nil {
		return nil, err
	}

	return account, nil
}

// GetAccount retrieves an account by ID.
// Returns ErrAccountNotFound if the account does not exist.
func (s *Service) GetAccount(id string) (*Account, error) {
	account, err := s.repo.FindByID(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	return account, nil
}

// DepositInput holds the validated input for a deposit operation.
type DepositInput struct {
	AccountID string
	// Amount is in minor units (pesewas/cents). Must be greater than zero.
	Amount int64
}

// Deposit credits the given amount to the account's balance and writes
// a CREDIT ledger entry, all within a single transaction.
func (s *Service) Deposit(input DepositInput) (*Account, error) {
	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	var updated *Account

	err := s.repo.db.Transaction(func(tx *gorm.DB) error {
		// Lock the row to prevent concurrent modifications
		acc, err := s.repo.FindByIDForUpdate(tx, input.AccountID)
		if err != nil {
			if IsNotFound(err) {
				return ErrAccountNotFound
			}
			return err
		}

		if acc.Status != AccountStatusActive {
			return ErrAccountNotActive
		}

		// Credit the balance
		acc.Balance += input.Amount

		if err := s.repo.UpdateBalance(tx, acc); err != nil {
			return err
		}

		// Write the ledger entry inside the same transaction.
		// If this fails the balance update also rolls back, atomicity guaranteed.
		if s.ledger != nil {
			if err := s.ledger.RecordDeposit(tx, acc.ID, input.Amount, acc.Balance); err != nil {
				return err
			}
		}

		updated = acc
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updated, nil
}

// WithdrawInput holds the validated input for a withdrawal operation.
type WithdrawInput struct {
	AccountID string
	// Amount is in minor units (pesewas/cents). Must be greater than zero.
	Amount int64
}

// Withdraw debits the given amount from the account's balance and writes
// a DEBIT ledger entry, all within a single transaction.
func (s *Service) Withdraw(input WithdrawInput) (*Account, error) {
	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	var updated *Account

	err := s.repo.db.Transaction(func(tx *gorm.DB) error {
		// Lock the row before reading the balance
		acc, err := s.repo.FindByIDForUpdate(tx, input.AccountID)
		if err != nil {
			if IsNotFound(err) {
				return ErrAccountNotFound
			}
			return err
		}

		if acc.Status != AccountStatusActive {
			return ErrAccountNotActive
		}

		// Reject overdrafts
		if acc.Balance < input.Amount {
			return ErrInsufficientBalance
		}

		// Debit the balance
		acc.Balance -= input.Amount

		if err := s.repo.UpdateBalance(tx, acc); err != nil {
			return err
		}

		// Write the ledger entry inside the same transaction
		if s.ledger != nil {
			if err := s.ledger.RecordWithdrawal(tx, acc.ID, input.Amount, acc.Balance); err != nil {
				return err
			}
		}

		updated = acc
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updated, nil
}

// isValidCurrency checks whether the given currency is supported.
func isValidCurrency(c AccountCurrency) bool {
	switch c {
	case CurrencyGHS, CurrencyUSD, CurrencyEUR:
		return true
	default:
		return false
	}
}
