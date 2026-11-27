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

package verda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	validation "github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/internal/validation"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

const testInferenceModelJSON = `{
	"code": "kimi-k3",
	"slug": "kimi-k3",
	"endpoint_model": "moonshotai/kimi-k3",
	"name": "Kimi K3",
	"provider_id": "moonshotai",
	"provider_name": "Moonshot AI",
	"category": "text",
	"status": "available",
	"accept_new_subscriptions": true,
	"is_free": false,
	"dashboard_priority": null,
	"tagline": "Agentic coding",
	"description": null,
	"about": null,
	"tags": ["coding"],
	"uses": [],
	"logo_url": null,
	"input_modalities": ["text"],
	"output_modalities": ["text"],
	"supported_sampling_parameters": ["temperature"],
	"supported_features": ["tools"],
	"pricing": {
		"input": "0.0000006",
		"output": "0.0000025",
		"offer": {"input": {"list_price": "0.000001", "discount_percentage": 40, "valid_from": null, "valid_until": "2026-12-31T00:00:00.000Z"}}
	},
	"service_tiers": ["standard", "batch"],
	"service_tier_pricing": [{"service_tier": "batch", "pricing": {"input": "0.0000003"}}],
	"created_at": "2026-09-01T10:00:00.000Z",
	"updated_at": "2026-10-01T10:00:00.000Z",
	"context_length": 262144,
	"max_output_length": 32768,
	"request_rate_limit_per_min": null,
	"datacenters": [{"country_code": "FI"}],
	"total_parameters_b": 1040,
	"active_parameters_b": 32.5,
	"tokenizer": null,
	"weights_url": null,
	"license": "modified-mit",
	"license_url": null,
	"release_date": "2026-08-20",
	"knowledge_cutoff": null,
	"replacement_code": null,
	"reasoning": {"optional": true, "supported_efforts": ["low", "high"], "default_effort": null},
	"capabilities": null,
	"benchmarks": {
		"indices": [],
		"groups": [],
		"comparison": {"columns": ["Kimi K3", "Other"], "rows": [{"label": "AIME 2025", "values": [89.6, null]}]}
	}
}`

func TestInferenceService_ListModels(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodGet, "/inference/models", func(w http.ResponseWriter, _ *http.Request) {
		writeInferenceJSON(t, w, `{"data": [`+testInferenceModelJSON+`]}`)
	})

	models, err := client.Inference.ListModels(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	m := models[0]
	if m.EndpointModel != "moonshotai/kimi-k3" || m.Status != InferenceModelStatusAvailable {
		t.Errorf("unexpected model: %+v", m)
	}
	if m.ContextLength == nil || *m.ContextLength != 262144 {
		t.Errorf("expected context_length 262144, got %v", m.ContextLength)
	}
	if m.ActiveParametersB == nil || *m.ActiveParametersB != 32.5 {
		t.Errorf("expected active_parameters_b 32.5, got %v", m.ActiveParametersB)
	}
	if m.Pricing.Input != "0.0000006" || m.Pricing.Offer == nil || m.Pricing.Offer.Input.DiscountPercentage != 40 {
		t.Errorf("unexpected pricing: %+v", m.Pricing)
	}
	if m.Description != nil || m.Tagline == nil || *m.Tagline != "Agentic coding" {
		t.Errorf("unexpected nullable strings: description=%v tagline=%v", m.Description, m.Tagline)
	}
	rows := m.Benchmarks.Comparison.Rows
	if len(rows) != 1 || rows[0].Values[0] == nil || *rows[0].Values[0] != 89.6 || rows[0].Values[1] != nil {
		t.Errorf("expected comparison scores with a null, got %+v", rows)
	}
}

func TestInferenceService_GetModel(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodGet, "/inference/models/kimi-k3", func(w http.ResponseWriter, _ *http.Request) {
		writeInferenceJSON(t, w, testInferenceModelJSON[:len(testInferenceModelJSON)-1]+`,
			"quota_tier": 2, "quota_rpm": 600, "quota_tpm": 2000000,
			"service_tier_quota": [{"service_tier": "batch", "quota": {"rpm": null, "tpm": null, "max_enqueued_tokens": 50000000}}]
		}`)
	})

	t.Run("get model with project quota", func(t *testing.T) {
		model, err := client.Inference.GetModel(context.Background(), "kimi-k3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if model.Code != "kimi-k3" || model.QuotaTier != 2 || model.QuotaTPM != 2000000 {
			t.Errorf("unexpected model: %+v", model)
		}
		q := model.ServiceTierQuota[0].Quota
		if q.RPM != nil || q.MaxEnqueuedTokens == nil || *q.MaxEnqueuedTokens != 50000000 {
			t.Errorf("unexpected batch quota: %+v", q)
		}
	})

	t.Run("rejects an empty code", func(t *testing.T) {
		_, err := client.Inference.GetModel(context.Background(), "")
		assertRejectedBeforeRequest(t, err)
	})
}

func TestInferenceService_Subscriptions(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	subscriptionJSON := `{
		"project_id": "6d4f1c2e-1c43-4c4a-9a43-0b7c3b6a4c11",
		"model_code": "kimi-k3",
		"service_tier": "standard",
		"quota_tier": 1,
		"rpm": 60,
		"tpm": null,
		"created_at": "2026-10-01T10:00:00.000Z",
		"updated_at": "2026-10-01T10:00:00.000Z"
	}`

	mockServer.SetHandler(http.MethodGet, "/inference/subscriptions", func(w http.ResponseWriter, _ *http.Request) {
		writeInferenceJSON(t, w, "["+subscriptionJSON+"]")
	})
	mockServer.SetHandler(http.MethodGet, "/inference/subscriptions/status", func(w http.ResponseWriter, _ *http.Request) {
		writeInferenceJSON(t, w, `{"status": "suspended", "is_manual_suspend": false, "suspend_reason": "insufficient_balance", "public_comment": null, "has_minimum_balance": false}`)
	})
	var subscribed bool
	mockServer.SetHandler(http.MethodPut, "/inference/subscriptions/kimi-k3/standard", func(w http.ResponseWriter, r *http.Request) {
		subscribed = true
		if r.ContentLength > 0 {
			t.Errorf("expected no request body, got %d bytes", r.ContentLength)
		}
		writeInferenceJSON(t, w, subscriptionJSON)
	})
	var unsubscribed bool
	mockServer.SetHandler(http.MethodDelete, "/inference/subscriptions/kimi-k3/standard", func(w http.ResponseWriter, _ *http.Request) {
		unsubscribed = true
		w.WriteHeader(http.StatusNoContent)
	})
	mockServer.SetHandler(http.MethodPut, "/inference/subscriptions/closed-model/standard", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code": "model_not_open", "message": "Inference model ` + "`closed-model`" + ` is not open for subscriptions yet"}`))
	})

	ctx := context.Background()

	t.Run("list", func(t *testing.T) {
		subs, err := client.Inference.ListSubscriptions(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(subs) != 1 || subs[0].RPM == nil || *subs[0].RPM != 60 || subs[0].TPM != nil {
			t.Errorf("unexpected subscriptions: %+v", subs)
		}
	})

	t.Run("status", func(t *testing.T) {
		status, err := client.Inference.GetStatus(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Status != InferenceTenantStatusSuspended || status.SuspendReason == nil || *status.SuspendReason != InferenceSuspendReasonInsufficientBalance {
			t.Errorf("unexpected status: %+v", status)
		}
	})

	t.Run("subscribe", func(t *testing.T) {
		sub, err := client.Inference.Subscribe(ctx, "kimi-k3", InferenceServiceTierStandard)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !subscribed || sub.ModelCode != "kimi-k3" {
			t.Errorf("unexpected subscription: %+v", sub)
		}
	})

	t.Run("subscribe surfaces the API error code", func(t *testing.T) {
		_, err := client.Inference.Subscribe(ctx, "closed-model", InferenceServiceTierStandard)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict || apiErr.Code != "model_not_open" {
			t.Errorf("expected a 409 model_not_open APIError, got %v", err)
		}
	})

	t.Run("unsubscribe", func(t *testing.T) {
		if err := client.Inference.Unsubscribe(ctx, "kimi-k3", InferenceServiceTierStandard); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !unsubscribed {
			t.Error("expected DELETE to reach the server")
		}
	})

	t.Run("rejects an unknown service tier before the request", func(t *testing.T) {
		_, err := client.Inference.Subscribe(ctx, "kimi-k3", "premium")
		assertRejectedBeforeRequest(t, err)
		assertRejectedBeforeRequest(t, client.Inference.Unsubscribe(ctx, "kimi-k3", ""))
	})

	t.Run("rejects a model code that is not one path segment", func(t *testing.T) {
		_, err := client.Inference.Subscribe(ctx, "..", InferenceServiceTierStandard)
		assertRejectedBeforeRequest(t, err)
	})
}

func TestInferenceService_GetUsage(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	var query string
	mockServer.SetHandler(http.MethodGet, "/inference/usage", func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		writeInferenceJSON(t, w, `{
			"rows": [{"day": "2026-09-16", "model": "kimi-k3", "requests": 3, "input_tokens": 1200, "cache_read_tokens": 100, "output_tokens": 340}],
			"totals": {"requests": 3, "input_tokens": 1200, "cache_read_tokens": 100, "output_tokens": 340},
			"range": {"start": "2026-09-01T00:00:00.000Z", "end": "2026-09-30T00:00:00.000Z"}
		}`)
	})

	ctx := context.Background()

	t.Run("without filters", func(t *testing.T) {
		usage, err := client.Inference.GetUsage(ctx, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if query != "" {
			t.Errorf("expected no query, got %q", query)
		}
		if len(usage.Rows) != 1 || usage.Rows[0].Day != "2026-09-16" || usage.Rows[0].InputTokens != 1200 || usage.Totals.OutputTokens != 340 {
			t.Errorf("unexpected usage: %+v", usage)
		}
	})

	t.Run("with a range and key in UTC", func(t *testing.T) {
		helsinki := time.FixedZone("EET", 2*60*60)
		_, err := client.Inference.GetUsage(ctx, &GetInferenceUsageOptions{
			Start:    time.Date(2026, 9, 1, 2, 0, 0, 0, helsinki),
			End:      time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC),
			APIKeyID: "21",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "api_key_id=21&end=2026-09-15T00%3A00%3A00Z&start=2026-09-01T00%3A00%3A00Z"
		if query != want {
			t.Errorf("expected query %q, got %q", want, query)
		}
	})

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for name, end := range map[string]time.Time{
		"an end before the start":     start.AddDate(0, 0, -1),
		"an end equal to the start":   start,
		"an end in the same second":   start.Add(500 * time.Millisecond),
		"a range longer than 31 days": start.Add(inferenceUsageMaxRange + time.Second),
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			query = "not requested"
			_, err := client.Inference.GetUsage(ctx, &GetInferenceUsageOptions{Start: start, End: end})
			var verrs validation.Errors
			if !errors.As(err, &verrs) || verrs["End"] == nil {
				t.Errorf("expected a validation error for End, got %v", err)
			}
			if query != "not requested" {
				t.Errorf("expected no request, got query %q", query)
			}
		})
	}

	t.Run("accepts a range of exactly 31 days", func(t *testing.T) {
		_, err := client.Inference.GetUsage(ctx, &GetInferenceUsageOptions{Start: start, End: start.Add(inferenceUsageMaxRange)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestInferenceService_RateLimitsAndTiers(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	tierJSON := `{"tier": 1, "rate_limits": [{"model_code": "kimi-k3", "service_tier": "standard", "rpm": 60, "tpm": 200000, "max_enqueued_tokens": null}]}`
	mockServer.SetHandler(http.MethodGet, "/inference/rate-limits", func(w http.ResponseWriter, _ *http.Request) {
		writeInferenceJSON(t, w, tierJSON)
	})
	mockServer.SetHandler(http.MethodGet, "/inference/tiers", func(w http.ResponseWriter, _ *http.Request) {
		writeInferenceJSON(t, w, "["+tierJSON+`, {"tier": 2, "rate_limits": []}]`)
	})

	ctx := context.Background()

	limits, err := client.Inference.GetRateLimits(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limits.Tier != 1 || len(limits.RateLimits) != 1 || *limits.RateLimits[0].TPM != 200000 || limits.RateLimits[0].MaxEnqueuedTokens != nil {
		t.Errorf("unexpected rate limits: %+v", limits)
	}

	tiers, err := client.Inference.ListTiers(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tiers) != 2 || tiers[1].Tier != 2 {
		t.Errorf("unexpected tiers: %+v", tiers)
	}
}

func writeInferenceJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("write mock response: %v", err)
	}
}

func TestInferenceService_APIKeys(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()
	client := NewTestClient(t, mockServer)

	mockServer.SetHandler(http.MethodGet, "/inference/api-keys", func(w http.ResponseWriter, _ *http.Request) {
		writeInferenceJSON(t, w, `[{"id": "key_1", "label": "my-agent", "masked_key": "dcs_53dc...833b", "created_at": "2026-11-18T10:00:00.000Z"}]`)
	})
	var createdLabel string
	mockServer.SetHandler(http.MethodPost, "/inference/api-keys", func(w http.ResponseWriter, r *http.Request) {
		var body CreateInferenceAPIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		createdLabel = body.Label
		w.WriteHeader(http.StatusCreated)
		writeInferenceJSON(t, w, `{"id": "key_2", "label": "`+body.Label+`", "masked_key": "dcs_aaaa...bbbb", "created_at": "2026-11-18T10:05:00.000Z", "key": "dcs_aaaa-secret-bbbb"}`)
	})
	var deleted bool
	mockServer.SetHandler(http.MethodDelete, "/inference/api-keys/key_2", func(w http.ResponseWriter, _ *http.Request) {
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	})
	mockServer.SetHandler(http.MethodDelete, "/inference/api-keys/gone", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code": "not_found", "message": "api key not found"}`))
	})

	ctx := context.Background()

	t.Run("list", func(t *testing.T) {
		keys, err := client.Inference.ListAPIKeys(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(keys) != 1 || keys[0].MaskedKey != "dcs_53dc...833b" || keys[0].CreatedAt.IsZero() {
			t.Errorf("unexpected keys: %+v", keys)
		}
	})

	t.Run("create returns the plaintext once", func(t *testing.T) {
		key, err := client.Inference.CreateAPIKey(ctx, &CreateInferenceAPIKeyRequest{Label: "verda-cli opencode"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if createdLabel != "verda-cli opencode" || key.ID != "key_2" || key.Key != "dcs_aaaa-secret-bbbb" {
			t.Errorf("unexpected created key: id=%s label=%s", key.ID, createdLabel)
		}
	})

	t.Run("create validates the label", func(t *testing.T) {
		for _, req := range []*CreateInferenceAPIKeyRequest{nil, {}, {Label: strings.Repeat("x", 129)}} {
			_, err := client.Inference.CreateAPIKey(ctx, req)
			assertRejectedBeforeRequest(t, err)
		}
		if err := (CreateInferenceAPIKeyRequest{Label: strings.Repeat("ä", 128)}).Validate(); err != nil {
			t.Errorf("128 characters should be accepted: %v", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		if err := client.Inference.DeleteAPIKey(ctx, "key_2"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !deleted {
			t.Error("expected DELETE to reach the server")
		}
	})

	t.Run("delete of a missing key surfaces 404", func(t *testing.T) {
		var apiErr *APIError
		if err := client.Inference.DeleteAPIKey(ctx, "gone"); !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("expected a 404 APIError, got %v", err)
		}
	})

	t.Run("delete refuses ids that are not one safe path segment", func(t *testing.T) {
		for _, id := range []string{"", "..", "a/b", "a.b", strings.Repeat("a", 65)} {
			assertRejectedBeforeRequest(t, client.Inference.DeleteAPIKey(ctx, id))
		}
	})
}

func TestCreatedInferenceAPIKey_FormatRedactsKey(t *testing.T) {
	const secret = "dcs_PLAINTEXT-SECRET"
	key := CreatedInferenceAPIKey{InferenceAPIKey: InferenceAPIKey{ID: "key_1", Label: "agent"}, Key: secret}

	outputs := map[string]string{
		"%v value":   fmt.Sprintf("%v", key),
		"%+v value":  fmt.Sprintf("%+v", key),
		"%#v value":  fmt.Sprintf("%#v", key),
		"%s value":   fmt.Sprintf("%s", key),
		"%v pointer": fmt.Sprintf("%v", &key),
		"%v slice":   fmt.Sprintf("%v", []CreatedInferenceAPIKey{key}),
		"%v wrapped": fmt.Errorf("failed: %v", key).Error(),
		"%q value":   fmt.Sprintf("%q", key),
		"%x value":   fmt.Sprintf("%x", key),
	}
	hexSecret := fmt.Sprintf("%x", secret)
	for name, out := range outputs {
		if strings.Contains(out, secret) || strings.Contains(out, hexSecret) {
			t.Errorf("%s leaks the key: %s", name, out)
		}
		if name != "%x value" && (!strings.Contains(out, "key_1") || !strings.Contains(out, "[REDACTED]")) {
			t.Errorf("%s should show the ID and a redaction marker, got: %s", name, out)
		}
	}
	if got := fmt.Sprintf("%#v", key); !strings.HasPrefix(got, "verda.CreatedInferenceAPIKey{") {
		t.Errorf("%%#v should name the public type, got %s", got)
	}

	body, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), secret) {
		t.Errorf("JSON must keep the key for callers that store it, got %s", body)
	}
}

// assertRejectedBeforeRequest fails unless err is a client-side validation error.
// The mock server answers unknown paths with an APIError, so a request that was
// sent anyway does not pass as a rejection.
func assertRejectedBeforeRequest(t *testing.T, err error) {
	t.Helper()
	var apiErr *APIError
	if err == nil || errors.As(err, &apiErr) {
		t.Errorf("expected a validation error before the request, got %v", err)
	}
}
