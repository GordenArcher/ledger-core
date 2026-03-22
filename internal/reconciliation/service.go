package reconciliation

import (
	"time"

	"github.com/GordenArcher/ledger-core/internal/account"
	"github.com/GordenArcher/ledger-core/internal/ledger"
	"gorm.io/gorm"
)

// AccountDiscrepancy represents a single account where the computed balance
// from ledger entries does not match the stored balance.
type AccountDiscrepancy struct {
	AccountID       string `json:"account_id"`
	OwnerName       string `json:"owner_name"`
	StoredBalance   int64  `json:"stored_balance"`   // what Account.Balance says
	ComputedBalance int64  `json:"computed_balance"` // sum of ledger entries
	Difference      int64  `json:"difference"`       // stored - computed
}

// Report is the full result of a reconciliation run.
type Report struct {
	// Timestamp of when the reconciliation was run
	RunAt time.Time `json:"run_at"`

	// Total number of accounts checked
	TotalAccounts int64 `json:"total_accounts"`

	// Sum of all credit entries across all accounts
	TotalCredits int64 `json:"total_credits"`

	// Sum of all debit entries across all accounts
	TotalDebits int64 `json:"total_debits"`

	// Sum of all current account balances
	TotalBalance int64 `json:"total_balance"`

	// The fundamental equation: total_credits - total_debits should equal total_balance.
	// If this is true, no money has been created or destroyed.
	Balanced bool `json:"balanced"`

	// Number of accounts with discrepancies
	DiscrepancyCount int `json:"discrepancy_count"`

	// List of accounts where stored balance != computed balance from ledger
	// Empty if everything is balanced.
	Discrepancies []AccountDiscrepancy `json:"discrepancies"`
}

// Service runs reconciliation checks across the ledger.
type Service struct {
	db *gorm.DB
}

// NewService creates a new reconciliation Service.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Run performs a full ledger reconciliation and returns a report.
//
// The core invariant of double-entry bookkeeping:
//
//	total_credits, total_debits == sum of all account balances
//
// If this doesn't hold, money has either been created or destroyed somewhere —
// which should never happen in a correctly implemented system.
func (s *Service) Run() (*Report, error) {
	report := &Report{
		RunAt:         time.Now(),
		Discrepancies: []AccountDiscrepancy{},
	}

	// Count total accounts
	if err := s.db.Model(&account.Account{}).Count(&report.TotalAccounts).Error; err != nil {
		return nil, err
	}

	// Sum all credit entries across the entire ledger
	var totalCredits struct{ Sum int64 }
	if err := s.db.Model(&ledger.Entry{}).
		Select("COALESCE(SUM(amount), 0) as sum").
		Where("type = ?", ledger.EntryTypeCredit).
		Scan(&totalCredits).Error; err != nil {
		return nil, err
	}
	report.TotalCredits = totalCredits.Sum

	// Sum all debit entries across the entire ledger
	var totalDebits struct{ Sum int64 }
	if err := s.db.Model(&ledger.Entry{}).
		Select("COALESCE(SUM(amount), 0) as sum").
		Where("type = ?", ledger.EntryTypeDebit).
		Scan(&totalDebits).Error; err != nil {
		return nil, err
	}
	report.TotalDebits = totalDebits.Sum

	// Sum all current account balances
	var totalBalance struct{ Sum int64 }
	if err := s.db.Model(&account.Account{}).
		Select("COALESCE(SUM(balance), 0) as sum").
		Scan(&totalBalance).Error; err != nil {
		return nil, err
	}
	report.TotalBalance = totalBalance.Sum

	// The fundamental check:
	// Every credit increases a balance, every debit decreases it.
	// So credits, debits must equal the total balance held across all accounts.
	report.Balanced = (report.TotalCredits - report.TotalDebits) == report.TotalBalance

	// Per-account check: find accounts where the stored balance doesn't match
	// what you'd get by replaying all their ledger entries.
	discrepancies, err := s.findDiscrepancies()
	if err != nil {
		return nil, err
	}

	report.Discrepancies = discrepancies
	report.DiscrepancyCount = len(discrepancies)

	return report, nil
}

// findDiscrepancies queries each account and compares its stored balance
// against the net of its ledger entries (credits - debits).
func (s *Service) findDiscrepancies() ([]AccountDiscrepancy, error) {
	// This query computes the expected balance for each account from the ledger,
	// then joins with the accounts table to compare against the stored balance.
	//
	// SUM(CASE WHEN type='credit' THEN amount ELSE -amount END) gives us
	// the net balance as seen by the ledger for each account.
	type row struct {
		AccountID       string
		OwnerName       string
		StoredBalance   int64
		ComputedBalance int64
	}

	var rows []row

	err := s.db.Raw(`
		SELECT
			a.id         AS account_id,
			a.owner_name AS owner_name,
			a.balance    AS stored_balance,
			COALESCE(SUM(
				CASE WHEN e.type = 'credit' THEN e.amount
				     ELSE -e.amount
				END
			), 0)        AS computed_balance
		FROM accounts a
		LEFT JOIN entries e ON e.account_id = a.id
		GROUP BY a.id, a.owner_name, a.balance
		HAVING a.balance != COALESCE(SUM(
			CASE WHEN e.type = 'credit' THEN e.amount
			     ELSE -e.amount
			END
		), 0)
	`).Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	discrepancies := make([]AccountDiscrepancy, len(rows))
	for i, r := range rows {
		discrepancies[i] = AccountDiscrepancy{
			AccountID:       r.AccountID,
			OwnerName:       r.OwnerName,
			StoredBalance:   r.StoredBalance,
			ComputedBalance: r.ComputedBalance,
			Difference:      r.StoredBalance - r.ComputedBalance,
		}
	}

	return discrepancies, nil
}
