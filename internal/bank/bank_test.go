package bank

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostingBalancesToZero(t *testing.T) {
	e := Entries("a", "b", 2500)
	require.Equal(t, int64(0), e[0].Delta+e[1].Delta)
	require.Equal(t, "DEBIT", e[0].Direction)
	require.Equal(t, "CREDIT", e[1].Direction)
}

func TestInsufficientFunds(t *testing.T) {
	_, err := NewBalance("a", 100, -2500)
	require.ErrorIs(t, err, ErrInsufficientFunds)
	nb, err := NewBalance(FundingAccountID, 0, -2500)
	require.NoError(t, err)
	require.Equal(t, int64(-2500), nb)
}
