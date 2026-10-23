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

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// dnsLabelPattern matches a DNS-1123 label: lowercase alphanumerics and '-',
// starting and ending with an alphanumeric. Length is checked separately.
var dnsLabelPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// KubernetesClusterPhase is the coarse lifecycle of a managed Kubernetes
// cluster. Unknown values are preserved as-is. Degraded can include a normal
// rollout, so it is not necessarily a terminal failure.
type KubernetesClusterPhase string

const (
	KubernetesClusterPhaseProvisioning KubernetesClusterPhase = "provisioning"
	KubernetesClusterPhaseRunning      KubernetesClusterPhase = "running"
	KubernetesClusterPhaseDegraded     KubernetesClusterPhase = "degraded"
	KubernetesClusterPhaseDeleting     KubernetesClusterPhase = "deleting"
	KubernetesClusterPhaseError        KubernetesClusterPhase = "error"
)

// Control-plane sizing presets. The server defaults to standard.
const (
	KubernetesControlPlaneProfileMinimal  = "minimal"
	KubernetesControlPlaneProfileStandard = "standard"
	KubernetesControlPlaneProfileLarge    = "large"
)

// KubernetesCluster represents a managed Kubernetes cluster. Clusters are
// identified by name within a project. KubernetesVersion is the requested
// version, not proof that the control plane runs it.
type KubernetesCluster struct {
	Name                string                 `json:"name"`
	KubernetesVersion   string                 `json:"kubernetes_version"`
	Location            string                 `json:"location"`
	NetworkID           string                 `json:"network"`
	SubnetID            string                 `json:"subnet"`
	ControlPlaneProfile string                 `json:"control_plane_profile,omitempty"`
	Phase               KubernetesClusterPhase `json:"phase"`
	// ExternalEndpoint is the API server URL; empty until published
	ExternalEndpoint string                `json:"external_endpoint,omitempty"`
	Conditions       []KubernetesCondition `json:"conditions,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
}

// KubernetesCondition is a per-component readiness state reported for a cluster
type KubernetesCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	ObservedGeneration int64     `json:"observedGeneration,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message"`
}

// CreateKubernetesClusterRequest represents the request to create a managed
// Kubernetes cluster. Location, network, subnet and CIDRs are immutable.
// Worker nodes are added separately through node pools.
type CreateKubernetesClusterRequest struct {
	Name              string `json:"name"`
	KubernetesVersion string `json:"kubernetes_version"`
	PodCIDR           string `json:"pod_cidr"`
	ServiceCIDR       string `json:"service_cidr"`
	Location          string `json:"location"`
	// NetworkID is the project's private network the workers attach to
	NetworkID string `json:"network"`
	// SubnetID must be in NetworkID and in Location
	SubnetID            string `json:"subnet"`
	ControlPlaneProfile string `json:"control_plane_profile,omitempty"`
}

// Validate validates the CreateKubernetesClusterRequest fields. Version
// availability, CIDR sizes and overlaps are checked by the server.
func (r CreateKubernetesClusterRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.Name,
			validation.Required,
			validation.RuneLength(1, 63),
			validation.Match(dnsLabelPattern).Error("must be a DNS-1123 label: lowercase alphanumerics or '-', starting and ending with an alphanumeric"),
		),
		validation.Field(&r.KubernetesVersion, validation.Required),
		validation.Field(&r.PodCIDR, validation.Required, validation.By(validateIPv4CIDR)),
		validation.Field(&r.ServiceCIDR, validation.Required, validation.By(validateIPv4CIDR)),
		validation.Field(&r.Location, validation.Required),
		validation.Field(&r.NetworkID, validation.Required),
		validation.Field(&r.SubnetID, validation.Required),
		validation.Field(&r.ControlPlaneProfile, validation.In(
			KubernetesControlPlaneProfileMinimal,
			KubernetesControlPlaneProfileStandard,
			KubernetesControlPlaneProfileLarge,
		)),
	)
}

// UpdateKubernetesClusterRequest requests a Kubernetes version upgrade, the
// only mutable cluster field
type UpdateKubernetesClusterRequest struct {
	KubernetesVersion string `json:"kubernetes_version"`
}

// Validate validates the UpdateKubernetesClusterRequest fields
func (r UpdateKubernetesClusterRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.KubernetesVersion, validation.Required),
	)
}

func validateKubernetesSelectors(projectID string, names ...string) error {
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
