package ledger

import (
	"fmt"

	"gorm.io/gorm"
)

// Service handles business logic for ledger entries.
type Service struct {
	repo *Repository
}

// NewService creates a new ledger Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// RecordDeposit writes a single CREDIT entry for a deposit operation.
// Must be called inside the same transaction as the balance update.
func (s *Service) RecordDeposit(tx *gorm.DB, accountID string, amount, balanceAfter int64) error {
	entry := &Entry{
		AccountID:    accountID,
		Type:         EntryTypeCredit,
		Source:       EntrySourceDeposit,
		Amount:       amount,
		BalanceAfter: balanceAfter,
		Description:  fmt.Sprintf("Deposit of %d", amount),
	}
	return s.repo.Create(tx, entry)
}

// RecordWithdrawal writes a single DEBIT entry for a withdrawal operation.
// Must be called inside the same transaction as the balance update.
func (s *Service) RecordWithdrawal(tx *gorm.DB, accountID string, amount, balanceAfter int64) error {
	entry := &Entry{
		AccountID:    accountID,
		Type:         EntryTypeDebit,
		Source:       EntrySourceWithdraw,
		Amount:       amount,
		BalanceAfter: balanceAfter,
		Description:  fmt.Sprintf("Withdrawal of %d", amount),
	}
	return s.repo.Create(tx, entry)
}

// RecordTransfer writes two ledger entries for a transfer, one debit and one credit.
// Both entries share the same TransferID so they can be linked.
// Must be called inside the same transaction as the balance updates.
func (s *Service) RecordTransfer(
	tx *gorm.DB,
	transferID string,
	fromAccountID string,
	toAccountID string,
	amount int64,
	senderBalanceAfter int64,
	receiverBalanceAfter int64,
	reference string,
) error {
	description := reference
	if description == "" {
		description = "Transfer"
	}

	// DEBIT entry on the sender's account
	debit := &Entry{
		AccountID:    fromAccountID,
		Type:         EntryTypeDebit,
		Source:       EntrySourceTransfer,
		Amount:       amount,
		BalanceAfter: senderBalanceAfter,
		TransferID:   &transferID,
		Description:  fmt.Sprintf("Transfer out: %s", description),
	}
	if err := s.repo.Create(tx, debit); err != nil {
		return err
	}

	// CREDIT entry on the receiver's account
	credit := &Entry{
		AccountID:    toAccountID,
		Type:         EntryTypeCredit,
		Source:       EntrySourceTransfer,
		Amount:       amount,
		BalanceAfter: receiverBalanceAfter,
		TransferID:   &transferID,
		Description:  fmt.Sprintf("Transfer in: %s", description),
	}
	return s.repo.Create(tx, credit)
}

// GetAccountLedger returns the full ledger history for an account.
func (s *Service) GetAccountLedger(accountID string) ([]Entry, error) {
	return s.repo.FindByAccount(accountID)
}

// GetAccountLedgerPaginated returns a page of ledger entries with total count.
func (s *Service) GetAccountLedgerPaginated(accountID string, page, pageSize int) ([]Entry, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.FindByAccountPaginated(accountID, offset, pageSize)
}
