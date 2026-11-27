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
	"errors"
	"net/url"
	"time"
)

const inferenceBasePath = "/inference"

// InferenceService manages Verda Inference for the project of the client's
// credentials. Every route except ListTiers answers 404 while inference is not
// enabled for that project.
type InferenceService struct {
	client *Client
}

// ListModels returns the inference model catalog.
func (s *InferenceService) ListModels(ctx context.Context) ([]InferenceModel, error) {
	resp, _, err := getRequest[struct {
		Data []InferenceModel `json:"data"`
	}](ctx, s.client, inferenceBasePath+"/models")
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetModel returns a model by its code, with the project's rate limits for it.
func (s *InferenceService) GetModel(ctx context.Context, code string) (*InferenceModelDetail, error) {
	path, err := joinPath(inferenceBasePath+"/models", "code", code, "")
	if err != nil {
		return nil, err
	}
	model, _, err := getRequest[InferenceModelDetail](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// ListSubscriptions returns the project's model subscriptions.
func (s *InferenceService) ListSubscriptions(ctx context.Context) ([]InferenceSubscription, error) {
	subscriptions, _, err := getRequest[[]InferenceSubscription](ctx, s.client, inferenceBasePath+"/subscriptions")
	if err != nil {
		return nil, err
	}
	return subscriptions, nil
}

// GetStatus returns whether inference is active or suspended for the project.
func (s *InferenceService) GetStatus(ctx context.Context) (*InferenceTenantStatus, error) {
	status, _, err := getRequest[InferenceTenantStatus](ctx, s.client, inferenceBasePath+"/subscriptions/status")
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// Subscribe subscribes the project to a model in a service tier. Subscribing
// again to the same model and tier is allowed and keeps one subscription.
func (s *InferenceService) Subscribe(ctx context.Context, modelCode, serviceTier string) (*InferenceSubscription, error) {
	path, err := subscriptionPath(modelCode, serviceTier)
	if err != nil {
		return nil, err
	}
	subscription, _, err := putRequest[InferenceSubscription](ctx, s.client, path, nil)
	if err != nil {
		return nil, err
	}
	return &subscription, nil
}

// Unsubscribe cancels the project's subscription to a model in a service tier.
func (s *InferenceService) Unsubscribe(ctx context.Context, modelCode, serviceTier string) error {
	path, err := subscriptionPath(modelCode, serviceTier)
	if err != nil {
		return err
	}
	_, err = deleteRequestAllowEmptyResponse(ctx, s.client, path)
	return err
}

// GetUsage returns the project's token usage per day and model; nil opts uses
// the API's default range.
func (s *InferenceService) GetUsage(ctx context.Context, opts *GetInferenceUsageOptions) (*InferenceUsage, error) {
	path := inferenceBasePath + "/usage"
	if opts != nil {
		if err := opts.Validate(); err != nil {
			return nil, err
		}
		params := url.Values{}
		if !opts.Start.IsZero() {
			params.Set("start", opts.Start.UTC().Format(time.RFC3339))
		}
		if !opts.End.IsZero() {
			params.Set("end", opts.End.UTC().Format(time.RFC3339))
		}
		if opts.APIKeyID != "" {
			params.Set("api_key_id", opts.APIKeyID)
		}
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
	}
	usage, _, err := getRequest[InferenceUsage](ctx, s.client, path)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

// GetRateLimits returns the project's quota tier and its rate limits.
func (s *InferenceService) GetRateLimits(ctx context.Context) (*InferenceTier, error) {
	limits, _, err := getRequest[InferenceTier](ctx, s.client, inferenceBasePath+"/rate-limits")
	if err != nil {
		return nil, err
	}
	return &limits, nil
}

// ListTiers returns the rate limits of every quota tier.
func (s *InferenceService) ListTiers(ctx context.Context) ([]InferenceTier, error) {
	tiers, _, err := getRequest[[]InferenceTier](ctx, s.client, inferenceBasePath+"/tiers")
	if err != nil {
		return nil, err
	}
	return tiers, nil
}

// ListAPIKeys returns the project's inference API keys, masked.
func (s *InferenceService) ListAPIKeys(ctx context.Context) ([]InferenceAPIKey, error) {
	keys, _, err := getRequest[[]InferenceAPIKey](ctx, s.client, inferenceBasePath+"/api-keys")
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// CreateAPIKey creates an inference API key. The returned Key is the only
// copy of the plaintext; the API cannot return it again.
func (s *InferenceService) CreateAPIKey(ctx context.Context, req *CreateInferenceAPIKeyRequest) (*CreatedInferenceAPIKey, error) {
	if req == nil {
		return nil, errors.New("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	key, _, err := postRequest[CreatedInferenceAPIKey](ctx, s.client, inferenceBasePath+"/api-keys", req)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

// DeleteAPIKey deletes an inference API key; requests made with it are rejected afterwards.
func (s *InferenceService) DeleteAPIKey(ctx context.Context, apiKeyID string) error {
	if err := validateInferenceAPIKeyID(apiKeyID); err != nil {
		return err
	}
	_, err := deleteRequestAllowEmptyResponse(ctx, s.client, inferenceBasePath+"/api-keys/"+apiKeyID)
	return err
}

func subscriptionPath(modelCode, serviceTier string) (string, error) {
	if err := validateInferenceServiceTier(serviceTier); err != nil {
		return "", err
	}
	return joinPath(inferenceBasePath+"/subscriptions", "modelCode", modelCode, "/"+serviceTier)
}
