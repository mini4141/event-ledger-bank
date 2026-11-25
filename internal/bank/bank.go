// Package bank holds pure domain rules shared by services.
package bank

import "errors"

// FundingAccountID is the system account that backs test deposits.
const FundingAccountID = "00000000-0000-0000-0000-00000000f00d"

// Entry is one side of a double-entry posting; Delta is signed (debit < 0).
type Entry struct {
	AccountID    string
	Direction    string // DEBIT | CREDIT
	AmountMinor  int64
	Delta        int64
	Counterparty string
}

var ErrInsufficientFunds = errors.New("insufficient funds")

// Entries returns the debit and credit entries for a posting; they always sum to zero.
func Entries(from, to string, amount int64) [2]Entry {
	return [2]Entry{
		{AccountID: from, Direction: "DEBIT", AmountMinor: amount, Delta: -amount, Counterparty: to},
		{AccountID: to, Direction: "CREDIT", AmountMinor: amount, Delta: amount, Counterparty: from},
	}
}

// NewBalance applies delta, refusing to overdraw unless the account is the funding account.
func NewBalance(accountID string, balance, delta int64) (int64, error) {
	nb := balance + delta
	if nb < 0 && accountID != FundingAccountID {
		return 0, ErrInsufficientFunds
	}
	return nb, nil
}
