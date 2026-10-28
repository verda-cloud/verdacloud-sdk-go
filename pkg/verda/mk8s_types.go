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
	"regexp"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// MK8SClusterNameMaxLength is the maximum length of a cluster name.
const MK8SClusterNameMaxLength = 30

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// dnsLabelPattern matches a DNS-1123 label: lowercase alphanumerics and '-',
// starting and ending with an alphanumeric. Validation checks length separately.
var dnsLabelPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// MK8SClusterPhase is the coarse lifecycle of a managed Kubernetes
// cluster. The SDK preserves unknown values. Degraded can include a normal
// rollout, so it does not necessarily indicate a terminal failure.
type MK8SClusterPhase string

const (
	MK8SClusterPhaseProvisioning MK8SClusterPhase = "provisioning"
	MK8SClusterPhaseRunning      MK8SClusterPhase = "running"
	MK8SClusterPhaseDegraded     MK8SClusterPhase = "degraded"
	MK8SClusterPhaseDeleting     MK8SClusterPhase = "deleting"
)

// MK8SCluster represents a managed Kubernetes cluster. Names identify clusters
// within a project. KubernetesVersion contains the requested version.
// It does not prove that the control plane runs that version.
type MK8SCluster struct {
	Name              string           `json:"name"`
	KubernetesVersion string           `json:"kubernetes_version"`
	Location          string           `json:"location"`
	NetworkID         string           `json:"network"`
	SubnetID          string           `json:"subnet"`
	Phase             MK8SClusterPhase `json:"phase"`
	// ExternalEndpoint is the API server URL. It is empty until the API publishes it.
	ExternalEndpoint string          `json:"external_endpoint,omitempty"`
	Conditions       []MK8SCondition `json:"conditions,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

// MK8SCondition is a per-component readiness state reported for a cluster
type MK8SCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	ObservedGeneration int64     `json:"observedGeneration,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message"`
}

// CreateMK8SClusterRequest represents the request to create a managed
// Kubernetes cluster. Location, network, subnet and CIDRs are immutable.
// Worker nodes require separate node pools.
type CreateMK8SClusterRequest struct {
	// Name must be a DNS-1123 label of 1 to 30 characters.
	Name              string `json:"name"`
	KubernetesVersion string `json:"kubernetes_version"`
	PodCIDR           string `json:"pod_cidr"`
	ServiceCIDR       string `json:"service_cidr"`
	Location          string `json:"location"`
	// NetworkID is the project's private network the workers attach to
	NetworkID string `json:"network"`
	// SubnetID must be in NetworkID and in Location
	SubnetID string `json:"subnet"`
}

// Validate checks the CreateMK8SClusterRequest fields. The server checks
// version availability, CIDR sizes, and CIDR overlaps.
func (r CreateMK8SClusterRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name,
			validation.Required,
			validation.RuneLength(1, MK8SClusterNameMaxLength),
			validation.Match(dnsLabelPattern).Error("must be a DNS-1123 label: lowercase alphanumerics or '-', starting and ending with an alphanumeric"),
		),
		validation.Field(&r.KubernetesVersion, validation.Required),
		validation.Field(&r.PodCIDR, validation.Required, validation.By(validateIPv4CIDR)),
		validation.Field(&r.ServiceCIDR, validation.Required, validation.By(validateIPv4CIDR)),
		validation.Field(&r.Location, validation.Required),
		validation.Field(&r.NetworkID, validation.Required),
		validation.Field(&r.SubnetID, validation.Required),
	)
}

// UpdateMK8SClusterRequest requests a Kubernetes version upgrade, the
// only mutable cluster field
type UpdateMK8SClusterRequest struct {
	KubernetesVersion string `json:"kubernetes_version"`
}

// Validate validates the UpdateMK8SClusterRequest fields
func (r UpdateMK8SClusterRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.KubernetesVersion, validation.Required),
	)
}

func validateMK8SSelectors(projectID string, names ...string) error {
	if err := validation.Validate(projectID, validation.Required, validation.Match(uuidPattern).Error("must be a valid UUID")); err != nil {
		return &ValidationError{Field: "projectID", Message: err.Error()}
	}
	for _, name := range names {
		if err := validation.Validate(name, validation.Required); err != nil {
			return &ValidationError{Field: "name", Message: err.Error()}
		}
	}
	return nil
}
