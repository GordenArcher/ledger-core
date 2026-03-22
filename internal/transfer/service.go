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

// LedgerRecorder is the interface the transfer service uses to write ledger entries.
// Same interface pattern as in the account service — avoids import cycles.
type LedgerRecorder interface {
	RecordTransfer(
		tx *gorm.DB,
		transferID string,
		fromAccountID string,
		toAccountID string,
		amount int64,
		senderBalanceAfter int64,
		receiverBalanceAfter int64,
		reference string,
	) error
}

// Service handles the business logic for transfers.
type Service struct {
	repo        *Repository
	accountRepo *account.Repository
	ledger      LedgerRecorder
	db          *gorm.DB
}

// NewService creates a new transfer Service.
func NewService(db *gorm.DB, repo *Repository, accountRepo *account.Repository, ledger LedgerRecorder) *Service {
	return &Service{
		repo:        repo,
		accountRepo: accountRepo,
		ledger:      ledger,
		db:          db,
	}
}

// TransferInput holds the validated input for a transfer operation.
type TransferInput struct {
	FromAccountID  string
	ToAccountID    string
	Amount         int64
	Reference      string
	IdempotencyKey string
}

// Execute performs a double-entry transfer between two accounts.
//
// Steps inside a single transaction:
//  1. Lock both accounts in a consistent order (deadlock prevention)
//  2. Validate status and balance
//  3. Debit sender, credit receiver
//  4. Write two ledger entries (one debit, one credit)
//  5. Persist the transfer record
//
// If any step fails, the entire transaction rolls back.
func (s *Service) Execute(input TransferInput) (*Transfer, error) {
	if input.FromAccountID == input.ToAccountID {
		return nil, ErrSameAccount
	}

	if input.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	// Generate idempotency key if client didn't provide one
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = uuid.New().String()
	}

	// If client provided a key, check if this transfer was already processed
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
		// Always lock the smaller UUID first, consistent across all callers.
		firstID, secondID := lockOrder(input.FromAccountID, input.ToAccountID)

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

		if sender.Status != account.AccountStatusActive {
			return errors.New("sender account is not active")
		}
		if receiver.Status != account.AccountStatusActive {
			return errors.New("receiver account is not active")
		}

		if sender.Currency != receiver.Currency {
			return ErrCurrencyMismatch
		}

		if sender.Balance < input.Amount {
			return ErrInsufficientBalance
		}

		// Apply double-entry balance changes
		sender.Balance -= input.Amount
		if err := s.accountRepo.UpdateBalance(tx, sender); err != nil {
			return err
		}

		receiver.Balance += input.Amount
		if err := s.accountRepo.UpdateBalance(tx, receiver); err != nil {
			return err
		}

		// Persist the transfer record
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

		// Write ledger entries for both sides of the transfer
		if s.ledger != nil {
			if err := s.ledger.RecordTransfer(
				tx,
				transfer.ID,
				input.FromAccountID,
				input.ToAccountID,
				input.Amount,
				sender.Balance,
				receiver.Balance,
				input.Reference,
			); err != nil {
				return err
			}
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
func lockOrder(a, b string) (string, string) {
	if a < b {
		return a, b
	}
	return b, a
}
