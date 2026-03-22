package account

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// Service errors, returned by the service layer and handled in the handler.
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

// Service contains the business logic for account operations.
// It sits between the HTTP handler and the repository.
type Service struct {
	repo *Repository
}

// NewService creates a new account Service with the given repository.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
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

	// Validate currency — only allow known values
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

// Deposit credits the given amount to the account's balance.
//
// Uses SELECT FOR UPDATE inside a transaction to prevent race conditions
// when multiple deposits hit the same account concurrently.
// This is the same pattern used in the ECG Credit Union system.
func (s *Service) Deposit(input DepositInput) (*Account, error) {
	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	var updated *Account

	// Wrap the read-modify-write in a transaction so no other operation
	// can modify the balance between our SELECT and UPDATE
	err := s.repo.db.Transaction(func(tx *gorm.DB) error {
		// Lock the row — blocks any concurrent deposit/withdraw/transfer
		// on this account until this transaction commits or rolls back
		acc, err := s.repo.FindByIDForUpdate(tx, input.AccountID)
		if err != nil {
			if IsNotFound(err) {
				return ErrAccountNotFound
			}
			return err
		}

		// Reject operations on non-active accounts
		if acc.Status != AccountStatusActive {
			return ErrAccountNotActive
		}

		// Credit the balance
		acc.Balance += input.Amount

		if err := s.repo.UpdateBalance(tx, acc); err != nil {
			return err
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

// Withdraw debits the given amount from the account's balance.
//
// Returns ErrInsufficientBalance if the account does not have enough funds.
// Uses SELECT FOR UPDATE inside a transaction to prevent race conditions
// where two concurrent withdrawals could both pass the balance check
// and together overdraft the account.
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

		// Reject the withdrawal if balance would go negative.
		// Financial systems never allow negative balances without explicit overdraft facilities.
		if acc.Balance < input.Amount {
			return ErrInsufficientBalance
		}

		// Debit the balance
		acc.Balance -= input.Amount

		if err := s.repo.UpdateBalance(tx, acc); err != nil {
			return err
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
