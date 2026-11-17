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
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Lifecycle phases of a ManagedKubernetesCluster.
const (
	ManagedKubernetesClusterPhaseProvisioning = "provisioning"
	ManagedKubernetesClusterPhaseRunning      = "running"
	ManagedKubernetesClusterPhaseDegraded     = "degraded"
	ManagedKubernetesClusterPhaseDeleting     = "deleting"
)

// ManagedKubernetesCondition is a per-component readiness state of a cluster.
// Field names keep the Kubernetes conditions JSON shape (camelCase).
type ManagedKubernetesCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	ObservedGeneration int       `json:"observedGeneration,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason"`
	Message            string    `json:"message"`
}

// ManagedKubernetesCluster represents a managed Kubernetes cluster.
type ManagedKubernetesCluster struct {
	Name              string                       `json:"name"`
	KubernetesVersion string                       `json:"kubernetes_version"`
	Location          string                       `json:"location"`
	NetworkID         string                       `json:"network"`
	SubnetID          string                       `json:"subnet"`
	PodCIDR           string                       `json:"pod_cidr"`
	ServiceCIDR       string                       `json:"service_cidr"`
	DNSSuffix         string                       `json:"dns_suffix,omitempty"`
	Phase             string                       `json:"phase"`
	ExternalEndpoint  string                       `json:"external_endpoint,omitempty"`
	Conditions        []ManagedKubernetesCondition `json:"conditions,omitempty"`
	CreatedAt         time.Time                    `json:"created_at"`
}

// CreateManagedKubernetesClusterRequest is the body of POST /mk8sclusters.
// NetworkID, SubnetID, PodCIDR, ServiceCIDR and Location are immutable
// once the cluster exists.
type CreateManagedKubernetesClusterRequest struct {
	// Name becomes the cluster identifier; a DNS-1123 label, max 30
	// characters on create.
	Name string `json:"name"`
	// KubernetesVersion must exist in the version catalogue.
	// Whitespace and a leading "v" are tolerated and stripped by the API.
	KubernetesVersion string `json:"kubernetes_version"`
	// PodCIDR is the pod IP range; prefix /21 or larger.
	PodCIDR string `json:"pod_cidr"`
	// ServiceCIDR is the service ClusterIP range; must not overlap PodCIDR.
	ServiceCIDR string `json:"service_cidr"`
	// Location must be enabled for the project and match the subnet's location.
	Location string `json:"location"`
	// NetworkID is the ID of the project's private network.
	NetworkID string `json:"network"`
	// SubnetID is the ID of the subnet within NetworkID; its CIDR must not
	// overlap PodCIDR or ServiceCIDR.
	SubnetID string `json:"subnet"`
}

func (r CreateManagedKubernetesClusterRequest) Validate() error {
	errs := fieldErrors{}
	errs.add("name", validateNewManagedKubernetesName(r.Name))
	errs.add("kubernetes_version", required(r.KubernetesVersion))
	errs.add("pod_cidr", firstErr(required(r.PodCIDR), validateIPv4CIDR(r.PodCIDR), validatePodCIDRPrefixLength(r.PodCIDR)))
	errs.add("service_cidr", firstErr(required(r.ServiceCIDR), validateIPv4CIDR(r.ServiceCIDR)))
	errs.add("location", required(r.Location))
	errs.add("network", required(r.NetworkID))
	errs.add("subnet", required(r.SubnetID))
	if err := errs.err(); err != nil {
		return err
	}
	return validateClusterCIDROverlap(r.PodCIDR, r.ServiceCIDR)
}

// UpdateManagedKubernetesClusterRequest is the body of PATCH
// /mk8sclusters/{location}/{clusterName}. KubernetesVersion is the only
// mutable field; the API runs the version change as a rolling update.
type UpdateManagedKubernetesClusterRequest struct {
	KubernetesVersion string `json:"kubernetes_version"`
}

func (r UpdateManagedKubernetesClusterRequest) Validate() error {
	errs := fieldErrors{}
	errs.add("kubernetes_version", required(r.KubernetesVersion))
	return errs.err()
}

// ManagedKubernetesEnabledLocations lists the sorted location codes the project may
// create managed Kubernetes clusters in.
type ManagedKubernetesEnabledLocations struct {
	Locations []string `json:"locations"`
}

// Lifecycle states of a ManagedKubernetesNodePool.
const (
	ManagedKubernetesNodePoolStateCreating = "creating"
	ManagedKubernetesNodePoolStateActive   = "active"
	ManagedKubernetesNodePoolStateDegraded = "degraded"
	ManagedKubernetesNodePoolStateDeleting = "deleting"
)

// Machine-readable reasons for a node pool error. New codes may be added by
// the API; treat unknown values as provisioning_failed.
const (
	ManagedKubernetesNodePoolErrorReasonInsufficientCapacity         = "insufficient_capacity"
	ManagedKubernetesNodePoolErrorReasonInsufficientFunds            = "insufficient_funds"
	ManagedKubernetesNodePoolErrorReasonInvalidCredentials           = "invalid_credentials" //nolint:gosec // G101: API error-reason string, not a credential
	ManagedKubernetesNodePoolErrorReasonInvalidInstanceConfiguration = "invalid_instance_configuration"
	ManagedKubernetesNodePoolErrorReasonCloudAPIError                = "cloud_api_error"
	ManagedKubernetesNodePoolErrorReasonProvisioningFailed           = "provisioning_failed"
)

// ManagedKubernetesNodePoolError reports why a pool is in an error state.
type ManagedKubernetesNodePoolError struct {
	Reason  string    `json:"reason"`
	Message string    `json:"message"`
	Since   time.Time `json:"since"`
}

// ManagedKubernetesNodePool represents a node pool of a managed Kubernetes
// cluster.
type ManagedKubernetesNodePool struct {
	Name          string                           `json:"name"`
	Replicas      int                              `json:"replicas"`
	ReadyReplicas int                              `json:"ready_replicas"`
	InstanceType  string                           `json:"instance_type"`
	SSHKeyIDs     []string                         `json:"ssh_key_ids,omitempty"`
	State         string                           `json:"state"`
	Errors        []ManagedKubernetesNodePoolError `json:"errors,omitempty"`
	CreatedAt     time.Time                        `json:"created_at"`
}

// CreateManagedKubernetesNodePoolRequest is the body of POST
// /mk8sclusters/{location}/{clusterName}/pools.
type CreateManagedKubernetesNodePoolRequest struct {
	// Name becomes the pool's name; a DNS-1123 label, max 30 characters on
	// create.
	Name string `json:"name"`
	// Replicas is the desired worker count; nil uses the API default (1).
	Replicas *int `json:"replicas,omitempty"`
	// InstanceType is the Verda VM type for the pool's workers.
	InstanceType string `json:"instance_type"`
	// SSHKeyIDs authorize the SSH keys injected into each worker (min 1).
	SSHKeyIDs []string `json:"ssh_key_ids"`
}

func (r CreateManagedKubernetesNodePoolRequest) Validate() error {
	errs := fieldErrors{}
	errs.add("name", validateNewManagedKubernetesName(r.Name))
	errs.add("replicas", validateReplicasIfSet(r.Replicas))
	errs.add("instance_type", required(r.InstanceType))
	errs.add("ssh_key_ids", requiredSlice(r.SSHKeyIDs))
	return errs.err()
}

// UpdateManagedKubernetesNodePoolRequest is the body of PATCH
// /mk8sclusters/{location}/{clusterName}/pools/{poolName}. Replicas is the
// only mutable field; 0 scales the pool down to zero.
type UpdateManagedKubernetesNodePoolRequest struct {
	Replicas *int `json:"replicas"`
}

func (r UpdateManagedKubernetesNodePoolRequest) Validate() error {
	errs := fieldErrors{}
	errs.add("replicas", firstErr(validateReplicasPresent(r.Replicas), validateReplicasIfSet(r.Replicas)))
	return errs.err()
}

// validateReplicasPresent requires a non-nil replicas pointer; unlike
// requiredPtr it accepts a pointed-at zero, which scales the pool to zero.
func validateReplicasPresent(replicas *int) error {
	if replicas == nil {
		return errBlank
	}
	return nil
}

// validateReplicasIfSet rejects a negative replica count; nil uses the API default.
func validateReplicasIfSet(replicas *int) error {
	if replicas == nil {
		return nil
	}
	return atLeast(*replicas, 0)
}

// Lifecycle statuses of a ManagedKubernetesKubeconfig.
const (
	ManagedKubernetesKubeconfigStatusPending = "pending"
	ManagedKubernetesKubeconfigStatusReady   = "ready"
)

// ManagedKubernetesKubeconfig represents a user kubeconfig of a managed
// Kubernetes cluster. Kubeconfig (the rendered YAML) is present only on
// single-GET once Status is ready; NotAfter is set at the same time.
type ManagedKubernetesKubeconfig struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name,omitempty"`
	CommonName  string     `json:"common_name"`
	Groups      []string   `json:"groups,omitempty"`
	Status      string     `json:"status"`
	NotAfter    *time.Time `json:"not_after,omitempty"`
	Kubeconfig  string     `json:"kubeconfig,omitempty"`
}

// String returns the kubeconfig record with the Kubeconfig YAML redacted;
// the YAML holds a client private key.
func (k ManagedKubernetesKubeconfig) String() string {
	return fmt.Sprintf("%+v", k.redacted())
}

// GoString is String in Go syntax, as used by %#v.
func (k ManagedKubernetesKubeconfig) GoString() string {
	s := fmt.Sprintf("%#v", k.redacted())
	return "verda.ManagedKubernetesKubeconfig" + strings.TrimPrefix(s, "verda.redactedManagedKubernetesKubeconfig")
}

// Format implements fmt.Formatter so that valid verbs, not only %v and %s,
// print the record with the Kubeconfig YAML redacted. fmt handles malformed
// directives such as %w outside fmt.Errorf, or %p on a non-pointer, before
// calling Format, so those can still print the raw fields.
func (k ManagedKubernetesKubeconfig) Format(f fmt.State, verb rune) {
	var s string
	switch {
	case verb == 'v' && f.Flag('#'):
		s = k.GoString()
	case verb == 'v' || verb == 's':
		s = k.String()
	default:
		s = fmt.Sprintf(fmt.FormatString(f, verb), k.redacted())
	}
	_, _ = io.WriteString(f, s) //nolint:errcheck // fmt.Formatter has no error return; fmt records write errors itself
}

// redactedManagedKubernetesKubeconfig has the fields of
// ManagedKubernetesKubeconfig but no fmt methods, so fmt prints its fields.
type redactedManagedKubernetesKubeconfig ManagedKubernetesKubeconfig

func (k ManagedKubernetesKubeconfig) redacted() redactedManagedKubernetesKubeconfig {
	if k.Kubeconfig != "" {
		k.Kubeconfig = "[REDACTED]"
	}
	return redactedManagedKubernetesKubeconfig(k)
}

// CreateManagedKubernetesKubeconfigRequest is the body of POST
// /mk8sclusters/{location}/{clusterName}/kubeconfigs.
type CreateManagedKubernetesKubeconfigRequest struct {
	// CommonName is the cert CN; control-plane identities ("system:*") are rejected.
	CommonName string `json:"common_name"`
	// Groups are the cert subject Os. "system:masters" grants cluster-admin;
	// any other "system:*" is rejected.
	Groups []string `json:"groups,omitempty"`
	// DisplayName is a free-form human-facing label (max 128 characters);
	// changing it never re-issues the cert.
	DisplayName string `json:"display_name,omitempty"`
	// Duration is the cert lifetime in Go duration syntax (e.g. "720h");
	// minimum 1 hour. Omitted defaults to 90 days.
	Duration string `json:"duration,omitempty"`
}

func (r CreateManagedKubernetesKubeconfigRequest) Validate() error {
	errs := fieldErrors{}
	errs.add("common_name", firstErr(required(r.CommonName), validateKubeconfigCommonName(r.CommonName)))
	errs.add("groups", validateEach(r.Groups, validateKubeconfigGroup))
	errs.add("display_name", firstErr(maxRuneLength(r.DisplayName, maxKubeconfigDisplayNameLength), validatePrintable(r.DisplayName)))
	errs.add("duration", validateKubeconfigDuration(r.Duration))
	return errs.err()
}

// maxKubeconfigDisplayNameLength is in characters (runes), not bytes.
const maxKubeconfigDisplayNameLength = 128

func validateKubeconfigCommonName(cn string) error {
	if strings.HasPrefix(strings.TrimSpace(cn), "system:") {
		return fmt.Errorf("%q is a control-plane identity", cn)
	}
	return nil
}

func validateKubeconfigGroup(group string) error {
	trimmed := strings.TrimSpace(group)
	if trimmed == "" {
		return fmt.Errorf("must not be empty")
	}
	if strings.HasPrefix(trimmed, "system:") && trimmed != "system:masters" {
		return fmt.Errorf("%q is a control-plane identity", trimmed)
	}
	return nil
}

func validatePrintable(s string) error {
	if strings.IndexFunc(s, func(r rune) bool { return !unicode.IsPrint(r) }) >= 0 {
		return fmt.Errorf("must contain printable characters only")
	}
	return nil
}

func validateKubeconfigDuration(s string) error {
	if s == "" {
		return nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q", s)
	}
	if d < time.Hour {
		return fmt.Errorf("duration must be at least 1h, got %q", s)
	}
	return nil
}

// Name length limits for clusters and node pools. The API accepts at most
// managedKubernetesNameMaxLength characters when a cluster or pool is
// created, and checks the length on create only. Paths of existing
// resources are therefore validated against the DNS-1123 label limit, so
// that resources with longer names stay reachable.
const (
	managedKubernetesNameMaxLength = 30
	dns1123LabelMaxLength          = 63
)

// validateManagedKubernetesName checks a cluster or pool name: a DNS-1123
// label of at most maxLen characters.
func validateManagedKubernetesName(field, value string, maxLen int) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if len(value) > maxLen {
		return fmt.Errorf("%s must be at most %d characters", field, maxLen)
	}
	if !dns1123LabelRegexp.MatchString(value) {
		return fmt.Errorf("%s %q must be a DNS-1123 label", field, value)
	}
	return nil
}

var dns1123LabelRegexp = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// validateNewManagedKubernetesName checks the name field of a create request.
func validateNewManagedKubernetesName(name string) error {
	return firstErr(required(name), lengthBetween(name, 1, managedKubernetesNameMaxLength), matches(name, dns1123LabelRegexp))
}

func validatePodCIDRPrefixLength(s string) error {
	prefix, err := netip.ParsePrefix(s)
	if err != nil {
		return nil // syntax is reported by the CIDR rule
	}
	if prefix.Bits() > 21 {
		return fmt.Errorf("must be /21 or larger, got %q", s)
	}
	return nil
}

func validateClusterCIDROverlap(podCIDR, serviceCIDR string) error {
	pod, podErr := netip.ParsePrefix(podCIDR)
	service, serviceErr := netip.ParsePrefix(serviceCIDR)
	if podErr != nil || serviceErr != nil {
		return nil // syntax is reported by the CIDR rules
	}
	if pod.Overlaps(service) {
		return fmt.Errorf("service_cidr must not overlap pod_cidr")
	}
	return nil
}
