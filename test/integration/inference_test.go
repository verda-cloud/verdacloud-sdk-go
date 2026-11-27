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

package integration

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda"
)

func inferenceAPIStatus(err error) int {
	var apiErr *verda.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}

// inferenceTestClient skips the test when inference is not enabled for the
// project of the credentials, which the API reports as 404.
func inferenceTestClient(t *testing.T) (*verda.Client, *verda.InferenceTenantStatus) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := getTestClient(t)
	status, err := client.Inference.GetStatus(context.Background())
	if inferenceAPIStatus(err) == http.StatusNotFound {
		t.Skip("inference is not enabled for the project")
	}
	if err != nil {
		t.Fatalf("failed to get inference status: %v", err)
	}
	return client, status
}

func TestInferenceIntegration(t *testing.T) {
	client, status := inferenceTestClient(t)
	ctx := context.Background()
	t.Logf("inference status: %s, minimum balance: %v", status.Status, status.HasMinimumBalance)

	t.Run("list_tiers", func(t *testing.T) {
		tiers, err := client.Inference.ListTiers(ctx)
		if err != nil {
			t.Fatalf("failed to list tiers: %v", err)
		}
		if len(tiers) == 0 {
			t.Fatal("expected at least one tier")
		}
	})

	t.Run("get_rate_limits", func(t *testing.T) {
		limits, err := client.Inference.GetRateLimits(ctx)
		if err != nil {
			t.Fatalf("failed to get rate limits: %v", err)
		}
		t.Logf("quota tier %d with %d limits", limits.Tier, len(limits.RateLimits))
	})

	var models []verda.InferenceModel
	t.Run("list_models", func(t *testing.T) {
		var err error
		models, err = client.Inference.ListModels(ctx)
		if err != nil {
			t.Fatalf("failed to list models: %v", err)
		}
		for _, m := range models {
			if m.Code == "" || m.EndpointModel == "" {
				t.Errorf("model without code or endpoint model: %+v", m)
			}
		}
		t.Logf("%d models", len(models))
	})

	t.Run("get_model", func(t *testing.T) {
		if len(models) == 0 {
			t.Skip("no models in the catalog")
		}
		model, err := client.Inference.GetModel(ctx, models[0].Code)
		if err != nil {
			t.Fatalf("failed to get model %s: %v", models[0].Code, err)
		}
		if model.Code != models[0].Code {
			t.Errorf("expected model %s, got %s", models[0].Code, model.Code)
		}
	})

	t.Run("get_unknown_model", func(t *testing.T) {
		_, err := client.Inference.GetModel(ctx, generateRandomName("sdk-missing-model"))
		if inferenceAPIStatus(err) != http.StatusNotFound {
			t.Errorf("expected 404, got %v", err)
		}
	})

	t.Run("list_subscriptions", func(t *testing.T) {
		if _, err := client.Inference.ListSubscriptions(ctx); err != nil {
			t.Fatalf("failed to list subscriptions: %v", err)
		}
	})

	t.Run("get_usage", func(t *testing.T) {
		if _, err := client.Inference.GetUsage(ctx, nil); err != nil {
			t.Fatalf("failed to get usage with the default range: %v", err)
		}

		end := time.Now().UTC().Truncate(time.Second)
		start := end.AddDate(0, 0, -7)
		usage, err := client.Inference.GetUsage(ctx, &verda.GetInferenceUsageOptions{Start: start, End: end})
		if err != nil {
			t.Fatalf("failed to get usage for a range: %v", err)
		}
		gotStart, err := time.Parse(time.RFC3339, usage.Range.Start)
		if err != nil || !gotStart.Equal(start) {
			t.Errorf("expected range start %s, got %q", start, usage.Range.Start)
		}
	})
}

// TestInferenceSubscriptionLifecycleIntegration subscribes to a model and tier
// the project is not subscribed to yet and cancels it again, so existing
// subscriptions are never touched. Subscribing is free; only usage is billed.
func TestInferenceSubscriptionLifecycleIntegration(t *testing.T) {
	client, status := inferenceTestClient(t)
	ctx := context.Background()

	if status.Status != verda.InferenceTenantStatusActive {
		t.Skipf("inference is %s for the project", status.Status)
	}
	if !status.HasMinimumBalance {
		t.Skip("the project balance is below the minimum needed to subscribe")
	}

	models, err := client.Inference.ListModels(ctx)
	if err != nil {
		t.Fatalf("failed to list models: %v", err)
	}
	existing, err := client.Inference.ListSubscriptions(ctx)
	if err != nil {
		t.Fatalf("failed to list subscriptions: %v", err)
	}
	subscribed := func(subs []verda.InferenceSubscription, code, tier string) bool {
		return slices.ContainsFunc(subs, func(s verda.InferenceSubscription) bool {
			return s.ModelCode == code && s.ServiceTier == tier
		})
	}

	tier := verda.InferenceServiceTierStandard
	i := slices.IndexFunc(models, func(m verda.InferenceModel) bool {
		return m.Status != verda.InferenceModelStatusComingSoon && m.AcceptNewSubscriptions &&
			slices.Contains(m.ServiceTiers, tier) && !subscribed(existing, m.Code, tier)
	})
	if i < 0 {
		t.Skip("no open model the project is not subscribed to yet")
	}
	code := models[i].Code

	sub, err := client.Inference.Subscribe(ctx, code, tier)
	switch inferenceAPIStatus(err) {
	case 0:
	case http.StatusPaymentRequired, http.StatusForbidden, http.StatusConflict:
		t.Skipf("cannot subscribe to %s: %v", code, err)
	default:
		t.Fatalf("failed to subscribe to %s: %v", code, err)
	}
	cancelled := false
	t.Cleanup(func() {
		if !cancelled {
			if err := client.Inference.Unsubscribe(context.Background(), code, tier); err != nil {
				t.Errorf("failed to cancel the subscription to %s: %v", code, err)
			}
		}
	})
	t.Logf("subscribed to %s (%s)", code, tier)

	if sub.ModelCode != code || sub.ServiceTier != tier {
		t.Errorf("unexpected subscription: %+v", sub)
	}

	again, err := client.Inference.Subscribe(ctx, code, tier)
	if err != nil {
		t.Fatalf("failed to subscribe again: %v", err)
	}
	if !again.CreatedAt.Equal(sub.CreatedAt) {
		t.Errorf("expected subscribing again to keep the subscription created at %s, got %s", sub.CreatedAt, again.CreatedAt)
	}

	subs, err := client.Inference.ListSubscriptions(ctx)
	if err != nil {
		t.Fatalf("failed to list subscriptions: %v", err)
	}
	if !subscribed(subs, code, tier) {
		t.Errorf("expected the subscription to %s to be listed", code)
	}

	if err := client.Inference.Unsubscribe(ctx, code, tier); err != nil {
		t.Fatalf("failed to unsubscribe: %v", err)
	}
	cancelled = true

	subs, err = client.Inference.ListSubscriptions(ctx)
	if err != nil {
		t.Fatalf("failed to list subscriptions: %v", err)
	}
	if subscribed(subs, code, tier) {
		t.Errorf("expected the subscription to %s to be gone", code)
	}

	if err := client.Inference.Unsubscribe(ctx, code, tier); inferenceAPIStatus(err) != http.StatusNotFound {
		t.Errorf("expected 404 when cancelling again, got %v", err)
	}
}
