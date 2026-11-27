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
	"errors"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// Service tiers an inference model can be subscribed to and called in.
const (
	InferenceServiceTierStandard = "standard"
	InferenceServiceTierFlex     = "flex"
	InferenceServiceTierBatch    = "batch"
	InferenceServiceTierFast     = "fast"
)

// InferenceServiceTiers lists every service tier the API accepts.
var InferenceServiceTiers = []string{
	InferenceServiceTierStandard,
	InferenceServiceTierFlex,
	InferenceServiceTierBatch,
	InferenceServiceTierFast,
}

// Availability of an inference model.
const (
	InferenceModelStatusComingSoon = "coming_soon"
	InferenceModelStatusBeta       = "beta"
	InferenceModelStatusAvailable  = "available"
)

// Whether inference is usable for a project.
const (
	InferenceTenantStatusActive    = "active"
	InferenceTenantStatusSuspended = "suspended"
)

// Why inference was suspended for a project.
const (
	InferenceSuspendReasonInsufficientBalance = "insufficient_balance"
	InferenceSuspendReasonByAdmin             = "by_admin"
	InferenceSuspendReasonUnknown             = "unknown"
)

// InferenceModel is a model of the inference catalog.
type InferenceModel struct {
	Code string `json:"code"`
	Slug string `json:"slug"`
	// EndpointModel is the value to send as `model` in inference requests.
	EndpointModel          string `json:"endpoint_model"`
	Name                   string `json:"name"`
	ProviderID             string `json:"provider_id"`
	ProviderName           string `json:"provider_name"`
	Category               string `json:"category"`
	Status                 string `json:"status"`
	AcceptNewSubscriptions bool   `json:"accept_new_subscriptions"`
	IsFree                 bool   `json:"is_free"`
	// DiscountToUser is the fraction of the published price charged, below 1.
	DiscountToUser              *float64 `json:"discount_to_user,omitempty"`
	DashboardPriority           *int     `json:"dashboard_priority"`
	Tagline                     *string  `json:"tagline"`
	Description                 *string  `json:"description"`
	About                       *string  `json:"about"`
	Tags                        []string `json:"tags"`
	Uses                        []string `json:"uses"`
	LogoURL                     *string  `json:"logo_url"`
	InputModalities             []string `json:"input_modalities"`
	OutputModalities            []string `json:"output_modalities"`
	SupportedSamplingParameters []string `json:"supported_sampling_parameters"`
	SupportedFeatures           []string `json:"supported_features"`
	// Pricing is the standard tier's price.
	Pricing            InferenceModelPricing         `json:"pricing"`
	ServiceTiers       []string                      `json:"service_tiers"`
	ServiceTierPricing []InferenceServiceTierPricing `json:"service_tier_pricing"`
	CreatedAt          time.Time                     `json:"created_at"`
	UpdatedAt          time.Time                     `json:"updated_at"`
	// ContextLength and MaxOutputLength are in tokens.
	ContextLength   *int64  `json:"context_length,omitempty"`
	MaxOutputLength *int64  `json:"max_output_length,omitempty"`
	Quantization    *string `json:"quantization,omitempty"`
	CapacityTPM     *int64  `json:"capacity_tpm,omitempty"`
	CapacityRPM     *int64  `json:"capacity_rpm,omitempty"`
	// Deprecated: Use CapacityRPM, which has the same value.
	RequestRateLimitPerMin *int64                     `json:"request_rate_limit_per_min"`
	Datacenters            []InferenceModelDatacenter `json:"datacenters,omitempty"`
	// TotalParametersB and ActiveParametersB are in billions.
	TotalParametersB  *float64 `json:"total_parameters_b"`
	ActiveParametersB *float64 `json:"active_parameters_b"`
	Tokenizer         *string  `json:"tokenizer"`
	HuggingFaceID     *string  `json:"hugging_face_id,omitempty"`
	WeightsURL        *string  `json:"weights_url"`
	License           *string  `json:"license"`
	LicenseURL        *string  `json:"license_url"`
	ReleaseDate       *string  `json:"release_date"`
	KnowledgeCutoff   *string  `json:"knowledge_cutoff"`
	DeprecationDate   *string  `json:"deprecation_date,omitempty"`
	// ReplacementCode is the code of the model that replaces a deprecated one.
	ReplacementCode *string                     `json:"replacement_code"`
	Reasoning       *InferenceModelReasoning    `json:"reasoning"`
	Capabilities    []InferenceModelSectionItem `json:"capabilities"`
	Benchmarks      *InferenceModelBenchmarks   `json:"benchmarks"`
}

// InferenceModelDetail is a model with the project's rate limits for it.
type InferenceModelDetail struct {
	InferenceModel
	QuotaTier        int                         `json:"quota_tier"`
	QuotaRPM         int64                       `json:"quota_rpm"`
	QuotaTPM         int64                       `json:"quota_tpm"`
	ServiceTierQuota []InferenceServiceTierQuota `json:"service_tier_quota"`
}

// InferenceModelPricing holds per-token prices as decimal strings in the
// customer's currency. Fields the model does not charge for are empty.
type InferenceModelPricing struct {
	Input      string                `json:"input,omitempty"`
	Output     string                `json:"output,omitempty"`
	CacheRead  string                `json:"cache_read,omitempty"`
	CacheWrite string                `json:"cache_write,omitempty"`
	Image      string                `json:"image,omitempty"`
	Request    string                `json:"request,omitempty"`
	Offer      *InferencePriceOffers `json:"offer,omitempty"`
}

// InferencePriceOffers holds the discounted prices that apply right now.
type InferencePriceOffers struct {
	Input     *InferencePriceOffer `json:"input,omitempty"`
	Output    *InferencePriceOffer `json:"output,omitempty"`
	CacheRead *InferencePriceOffer `json:"cache_read,omitempty"`
}

// InferencePriceOffer is a discount on one price.
type InferencePriceOffer struct {
	ListPrice          string  `json:"list_price"`
	DiscountPercentage float64 `json:"discount_percentage"`
	ValidFrom          *string `json:"valid_from"`
	ValidUntil         *string `json:"valid_until"`
}

// InferenceServiceTierPricing is a model's price in one service tier.
type InferenceServiceTierPricing struct {
	ServiceTier string                `json:"service_tier"`
	Pricing     InferenceModelPricing `json:"pricing"`
}

// InferenceModelDatacenter is a country a model is served from.
type InferenceModelDatacenter struct {
	CountryCode string `json:"country_code"`
}

// InferenceModelReasoning describes a model's reasoning controls.
type InferenceModelReasoning struct {
	Optional         bool     `json:"optional"`
	SupportedEfforts []string `json:"supported_efforts"`
	DefaultEffort    *string  `json:"default_effort,omitempty"`
}

// InferenceModelSectionItem is one entry of a model page section.
type InferenceModelSectionItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// InferenceModelBenchmarks holds a model's published benchmark results.
type InferenceModelBenchmarks struct {
	Indices    []InferenceModelBenchmarkIndex     `json:"indices"`
	Groups     []InferenceModelBenchmarkGroup     `json:"groups"`
	Comparison *InferenceModelBenchmarkComparison `json:"comparison,omitempty"`
}

// InferenceModelBenchmarkIndex is a headline benchmark score.
type InferenceModelBenchmarkIndex struct {
	Label      string  `json:"label"`
	Value      float64 `json:"value"`
	BetterThan float64 `json:"better_than"`
}

// InferenceModelBenchmarkGroup is a titled group of benchmark scores.
type InferenceModelBenchmarkGroup struct {
	Title string                        `json:"title"`
	Items []InferenceModelBenchmarkItem `json:"items"`
}

// InferenceModelBenchmarkItem is one benchmark score.
type InferenceModelBenchmarkItem struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Value       float64 `json:"value"`
}

// InferenceModelBenchmarkComparison compares a model with others, one column per model.
type InferenceModelBenchmarkComparison struct {
	Columns []string                               `json:"columns"`
	Rows    []InferenceModelBenchmarkComparisonRow `json:"rows"`
}

// InferenceModelBenchmarkComparisonRow holds one score per column; nil where none was published.
type InferenceModelBenchmarkComparisonRow struct {
	Label  string     `json:"label"`
	Values []*float64 `json:"values"`
}

// InferenceServiceTierQuota is the project's quota for a model in one service tier.
type InferenceServiceTierQuota struct {
	ServiceTier string         `json:"service_tier"`
	Quota       InferenceQuota `json:"quota"`
}

// InferenceQuota holds rate limits; nil means unlimited or not applicable.
type InferenceQuota struct {
	RPM               *int64 `json:"rpm"`
	TPM               *int64 `json:"tpm"`
	MaxEnqueuedTokens *int64 `json:"max_enqueued_tokens"`
}

// InferenceRateLimit is the limit of a model in one service tier.
type InferenceRateLimit struct {
	ModelCode         string `json:"model_code"`
	ServiceTier       string `json:"service_tier"`
	RPM               *int64 `json:"rpm"`
	TPM               *int64 `json:"tpm"`
	MaxEnqueuedTokens *int64 `json:"max_enqueued_tokens"`
}

// InferenceTier is a quota tier and the rate limits it grants.
type InferenceTier struct {
	Tier       int                  `json:"tier"`
	RateLimits []InferenceRateLimit `json:"rate_limits"`
}

// InferenceSubscription subscribes a project to a model in a service tier.
type InferenceSubscription struct {
	ProjectID   string    `json:"project_id"`
	ModelCode   string    `json:"model_code"`
	ServiceTier string    `json:"service_tier"`
	QuotaTier   int       `json:"quota_tier"`
	RPM         *int64    `json:"rpm"`
	TPM         *int64    `json:"tpm"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// InferenceTenantStatus tells whether inference is active or suspended for the project.
type InferenceTenantStatus struct {
	Status            string  `json:"status"`
	IsManualSuspend   bool    `json:"is_manual_suspend"`
	SuspendReason     *string `json:"suspend_reason"`
	PublicComment     *string `json:"public_comment"`
	HasMinimumBalance bool    `json:"has_minimum_balance"`
}

// InferenceUsage is the project's token usage per day and model.
type InferenceUsage struct {
	// Rows has one entry per day and model, newest day first.
	Rows   []InferenceUsageRow  `json:"rows"`
	Totals InferenceUsageTotals `json:"totals"`
	Range  InferenceUsageRange  `json:"range"`
}

// InferenceUsageTotals sums requests and tokens.
type InferenceUsageTotals struct {
	Requests        int64 `json:"requests"`
	InputTokens     int64 `json:"input_tokens"`
	CacheReadTokens int64 `json:"cache_read_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
}

// InferenceUsageRow is the usage of one model on one UTC day.
type InferenceUsageRow struct {
	InferenceUsageTotals
	// Day is a UTC calendar day, YYYY-MM-DD.
	Day   string `json:"day"`
	Model string `json:"model"`
}

// InferenceUsageRange is the time range the usage covers.
type InferenceUsageRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// GetInferenceUsageOptions filters GetUsage. Every field is optional.
type GetInferenceUsageOptions struct {
	// Start (inclusive) and End (exclusive) bound the range, at most 31 days
	// long; they are sent with second precision. A zero End means now, and a
	// zero Start means 30 days before End.
	Start time.Time
	End   time.Time
	// APIKeyID limits usage to one inference API key.
	APIKeyID string
}

// inferenceUsageMaxRange is the longest range GetUsage accepts.
const inferenceUsageMaxRange = 31 * 24 * time.Hour

// Validate validates the GetInferenceUsageOptions fields
func (opts GetInferenceUsageOptions) Validate() error {
	return validation.ValidateStruct(&opts,
		validation.Field(&opts.End, validation.When(!opts.Start.IsZero() && !opts.End.IsZero(),
			validation.By(func(any) error {
				// Compare what is sent, so a sub-second range is not accepted here and refused by the API
				start, end := opts.Start.Truncate(time.Second), opts.End.Truncate(time.Second)
				if !end.After(start) {
					return errors.New("must be after start")
				}
				if end.Sub(start) > inferenceUsageMaxRange {
					return errors.New("must be at most 31 days after start")
				}
				return nil
			}))),
	)
}

func validateInferenceServiceTier(serviceTier string) error {
	return validation.Validate(serviceTier,
		validation.Required.Error("serviceTier must not be empty"),
		validation.In(stringsToAny(InferenceServiceTiers)...).Error("serviceTier must be one of standard, flex, batch, fast"),
	)
}

func stringsToAny(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}
