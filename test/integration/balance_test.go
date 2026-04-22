// Copyright 2026 Verda Cloud Oy
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
			t.Fatalf("failed to get balance: %v", err)
		}
		if balance == nil {
			t.Fatal("expected balance information")
		}
		t.Logf("Account balance: %.2f %s", balance.Amount, balance.Currency)
	})
}
