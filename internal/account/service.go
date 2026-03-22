package account

import (
	"errors"
	"strings"
)

// Service errors — returned by the service layer and handled in the handler.
// Using typed errors lets the handler make decisions based on error type
// rather than string matching.
var (
	ErrAccountNotFound    = errors.New("account not found")
	ErrEmailAlreadyExists = errors.New("an account with this email already exists")
	ErrInvalidCurrency    = errors.New("unsupported currency, must be one of: GHS, USD, EUR")
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

// isValidCurrency checks whether the given currency is supported.
func isValidCurrency(c AccountCurrency) bool {
	switch c {
	case CurrencyGHS, CurrencyUSD, CurrencyEUR:
		return true
	default:
		return false
	}
}
