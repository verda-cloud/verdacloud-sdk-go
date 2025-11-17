//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
)

func TestBalance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	client := getTestClient(t)

	t.Run("get_balance", func(t *testing.T) {
		ctx := context.Background()
		balance, err := client.Balance.Get(ctx)
		if err != nil {
			t.Errorf("failed to get balance: %v", err)
		}
		if balance == nil {
			t.Error("expected balance information")
		}
		t.Logf("Account balance: %.2f %s", balance.Amount, balance.Currency)
	})
}
