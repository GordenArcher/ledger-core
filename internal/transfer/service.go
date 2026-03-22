package transfer

import (
	"errors"
	"fmt"

	"github.com/GordenArcher/ledger-core/internal/account"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service errors
var (
	ErrSameAccount         = errors.New("cannot transfer to the same account")
	ErrInvalidAmount       = errors.New("transfer amount must be greater than zero")
	ErrCurrencyMismatch    = errors.New("both accounts must use the same currency")
	ErrInsufficientBalance = errors.New("insufficient balance in source account")
	ErrTransferNotFound    = errors.New("transfer not found")
	ErrDuplicateTransfer   = errors.New("a transfer with this idempotency key already exists")
)

// Service handles the business logic for transfers.
// It depends on the account repository directly because a transfer
// must atomically modify two accounts, we need access to their rows.
type Service struct {
	repo        *Repository
	accountRepo *account.Repository
	db          *gorm.DB
}

// NewService creates a new transfer Service.
// db is passed in directly so the service can open transactions that
// span both the accounts table and the transfers table.
func NewService(db *gorm.DB, repo *Repository, accountRepo *account.Repository) *Service {
	return &Service{
		repo:        repo,
		accountRepo: accountRepo,
		db:          db,
	}
}

// TransferInput holds the validated input for a transfer operation.
type TransferInput struct {
	FromAccountID  string
	ToAccountID    string
	Amount         int64  // in minor units (pesewas/cents)
	Reference      string // optional description
	IdempotencyKey string // optional — enforced in Phase 6 middleware
}

// Execute performs a double-entry transfer between two accounts.
//
// Double-entry bookkeeping means every transfer produces two effects:
//   - A DEBIT on the sender   (balance decreases)
//   - A CREDIT on the receiver (balance increases)
//
// Both effects happen inside a single database transaction.
// If anything fails, insufficient funds, DB error, account frozen —
// the entire transaction rolls back and neither balance changes.
//
// Locking order: we always lock the account with the lexicographically
// smaller ID first. This prevents deadlocks when two concurrent transfers
// involve the same pair of accounts in opposite directions.
// e.g. A→B and B→A both try to lock A first, so one waits for the other.
func (s *Service) Execute(input TransferInput) (*Transfer, error) {
	// Basic validations before touching the DB
	if input.FromAccountID == input.ToAccountID {
		return nil, ErrSameAccount
	}

	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	// If no idempotency key was provided, generate one.
	// This ensures every transfer has a unique key and the DB constraint is never violated.
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = uuid.New().String()
	}

	// Idempotency check, if this key was already used, return the existing transfer
	if input.IdempotencyKey != "" {
		existing, err := s.repo.FindByIdempotencyKey(input.IdempotencyKey)
		if err == nil {
			// Key found, return the previously completed transfer
			return existing, nil
		}
		if !IsNotFound(err) {
			return nil, err
		}
		// IsNotFound → this is a new request, proceed normally
	}

	var completed *Transfer

	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Determine locking order to prevent deadlocks.
		// Always acquire locks in a consistent order (smaller UUID first).
		firstID, secondID := lockOrder(input.FromAccountID, input.ToAccountID)

		// Lock both accounts, no other transfer can touch them until we commit
		first, err := s.accountRepo.FindByIDForUpdate(tx, firstID)
		if err != nil {
			if account.IsNotFound(err) {
				return fmt.Errorf("account %s not found", firstID)
			}
			return err
		}

		second, err := s.accountRepo.FindByIDForUpdate(tx, secondID)
		if err != nil {
			if account.IsNotFound(err) {
				return fmt.Errorf("account %s not found", secondID)
			}
			return err
		}

		// Map back to sender/receiver regardless of lock order
		var sender, receiver *account.Account
		if first.ID == input.FromAccountID {
			sender, receiver = first, second
		} else {
			sender, receiver = second, first
		}

		// Both accounts must be active
		if sender.Status != account.AccountStatusActive {
			return errors.New("sender account is not active")
		}
		if receiver.Status != account.AccountStatusActive {
			return errors.New("receiver account is not active")
		}

		// Both accounts must share the same currency.
		// Cross-currency transfers would require an FX rate — out of scope here.
		if sender.Currency != receiver.Currency {
			return ErrCurrencyMismatch
		}

		// Sender must have enough funds
		if sender.Balance < input.Amount {
			return ErrInsufficientBalance
		}

		// Apply the double-entry:
		// DEBIT sender (balance goes down)
		sender.Balance -= input.Amount
		if err := s.accountRepo.UpdateBalance(tx, sender); err != nil {
			return err
		}

		// CREDIT receiver (balance goes up)
		receiver.Balance += input.Amount
		if err := s.accountRepo.UpdateBalance(tx, receiver); err != nil {
			return err
		}

		// Record the transfer
		transfer := &Transfer{
			FromAccountID:  input.FromAccountID,
			ToAccountID:    input.ToAccountID,
			Amount:         input.Amount,
			Currency:       string(sender.Currency),
			Reference:      input.Reference,
			Status:         TransferStatusCompleted,
			IdempotencyKey: input.IdempotencyKey,
		}

		if err := s.repo.Create(tx, transfer); err != nil {
			return err
		}

		completed = transfer
		return nil
	})

	if err != nil {
		return nil, err
	}

	return completed, nil
}

// GetTransfer retrieves a transfer by ID.
func (s *Service) GetTransfer(id string) (*Transfer, error) {
	transfer, err := s.repo.FindByID(id)
	if err != nil {
		if IsNotFound(err) {
			return nil, ErrTransferNotFound
		}
		return nil, err
	}
	return transfer, nil
}

// GetTransfersByAccount returns all transfers involving the given account.
func (s *Service) GetTransfersByAccount(accountID string) ([]Transfer, error) {
	return s.repo.FindByAccount(accountID)
}

// lockOrder returns two account IDs in a consistent order (smaller UUID first).
// This ensures concurrent transfers between the same pair always lock in the
// same order, preventing deadlocks.
func lockOrder(a, b string) (string, string) {
	if a < b {
		return a, b
	}
	return b, a
}
