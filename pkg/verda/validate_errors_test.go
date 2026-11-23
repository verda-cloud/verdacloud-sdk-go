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
	"math"
	"strings"
	"testing"
)

type validatable interface{ Validate() error }

// validateCase pins the exact Validate error text, which callers may log or match on.
type validateCase struct {
	name  string
	input validatable
	want  string // "" means the input is valid
}

func runValidateCases(t *testing.T, cases []validateCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			got := ""
			if err != nil {
				got = err.Error()
			}
			if got != tc.want {
				t.Errorf("Validate() error = %q, want %q", got, tc.want)
			}
		})
	}
}

func tagRequests(n int) []TagRequest {
	tags := make([]TagRequest, n)
	for i := range tags {
		tags[i] = TagRequest{Key: "k"}
	}
	return tags
}

func TestValidateErrors_SSHKeysAndStartupScripts(t *testing.T) {
	runValidateCases(t, []validateCase{
		{"ssh key valid", CreateSSHKeyRequest{Name: "k", PublicKey: "ssh-ed25519 AAAA"}, ""},
		{"ssh key empty", CreateSSHKeyRequest{}, "key: cannot be blank; name: cannot be blank."},
		{"startup script valid", CreateStartupScriptRequest{Name: "s", Script: "#!/bin/sh"}, ""},
		{"startup script empty", CreateStartupScriptRequest{}, "name: cannot be blank; script: cannot be blank."},
	})
}

func TestValidateErrors_Tags(t *testing.T) {
	runValidateCases(t, []validateCase{
		{"tag valid", TagRequest{Key: "env", Value: "prod"}, ""},
		{"tag key only", TagRequest{Key: "env"}, ""},
		{"tag key empty", TagRequest{Value: "prod"}, "key: cannot be blank."},
		{"tag key dot", TagRequest{Key: "."}, `key: must not be ".".`},
		{"tag key dot dot", TagRequest{Key: ".."}, `key: must not be "..".`},
		{"tag key at max", TagRequest{Key: strings.Repeat("k", TagKeyMaxLength)}, ""},
		{"tag key too long", TagRequest{Key: strings.Repeat("k", TagKeyMaxLength+1)}, "key: the length must be no more than 63."},
		{"tag value too long", TagRequest{Key: "k", Value: strings.Repeat("v", TagValueMaxLength+1)}, "value: the length must be no more than 127."},
		{"filter valid", TagFilter{Key: "env", Value: "prod"}, ""},
		{"filter key empty", TagFilter{}, "Key: cannot be blank."},
		{"filter key too long", TagFilter{Key: strings.Repeat("k", TagKeyMaxLength+1)}, "Key: the length must be no more than 63."},
		{"filter key with equals", TagFilter{Key: "a=b"}, `Key: must not contain '=', got "a=b".`},
		{"filter value too long", TagFilter{Key: "k", Value: strings.Repeat("v", TagValueMaxLength+1)}, "Value: the length must be no more than 127."},
		{"list options empty", ListInstancesOptions{}, ""},
		{"list options valid tags", ListInstancesOptions{Tags: []TagFilter{{Key: "env"}}}, ""},
		{"list options bad tag", ListInstancesOptions{Tags: []TagFilter{{Key: "ok"}, {}}}, "Tags: (1: (Key: cannot be blank.).)."},
	})
}

func TestValidateErrors_Clusters(t *testing.T) {
	valid := CreateClusterRequest{
		ClusterType: "16B200", Image: "ubuntu", Hostname: "h", Description: "d",
		SharedVolume: ClusterSharedVolumeSpec{Name: "sv", Size: 30000},
	}
	with := func(mut func(*CreateClusterRequest)) CreateClusterRequest {
		r := valid
		mut(&r)
		return r
	}
	runValidateCases(t, []validateCase{
		{"valid", valid, ""},
		{"empty", CreateClusterRequest{}, "cluster_type: cannot be blank; description: cannot be blank; hostname: cannot be blank; image: cannot be blank; shared_volume: cannot be blank."},
		{"contract valid", with(func(r *CreateClusterRequest) { r.Contract = "LONG_TERM" }), ""},
		{"contract invalid", with(func(r *CreateClusterRequest) { r.Contract = "SPOT" }), "contract: must be a valid value."},
		{"max tags", with(func(r *CreateClusterRequest) { r.Tags = tagRequests(MaxTagsPerResource) }), ""},
		{"too many tags", with(func(r *CreateClusterRequest) { r.Tags = tagRequests(MaxTagsPerResource + 1) }), "tags: the length must be no more than 10."},
		{"invalid tag", with(func(r *CreateClusterRequest) { r.Tags = []TagRequest{{}} }), "tags: (0: (key: cannot be blank.).)."},
		{"shared volume size blank", with(func(r *CreateClusterRequest) { r.SharedVolume.Size = 0 }), "shared_volume: (size: cannot be blank.)."},
		{"shared volume size negative", with(func(r *CreateClusterRequest) { r.SharedVolume.Size = -1 }), "shared_volume: (size: must be no less than 1.)."},
		{"shared volume name blank", with(func(r *CreateClusterRequest) { r.SharedVolume.Name = "" }), "shared_volume: (name: cannot be blank.)."},
	})
}

func TestValidateErrors_Volumes(t *testing.T) {
	valid := VolumeCreateRequest{Name: "v", Size: 50, Type: VolumeTypeNVMe}
	with := func(mut func(*VolumeCreateRequest)) VolumeCreateRequest {
		r := valid
		mut(&r)
		return r
	}
	runValidateCases(t, []validateCase{
		{"create valid", valid, ""},
		{"create empty", VolumeCreateRequest{}, "name: cannot be blank; size: cannot be blank; type: cannot be blank."},
		{"create size negative", with(func(r *VolumeCreateRequest) { r.Size = -1 }), "size: must be no less than 1."},
		{"create type invalid", with(func(r *VolumeCreateRequest) { r.Type = "SSD" }), "type: must be a valid value."},
		{"create too many tags", with(func(r *VolumeCreateRequest) { r.Tags = tagRequests(MaxTagsPerResource + 1) }), "tags: the length must be no more than 10."},
		{"create invalid tag", with(func(r *VolumeCreateRequest) { r.Tags = []TagRequest{{Key: "ok"}, {}} }), "tags: (1: (key: cannot be blank.).)."},
		{"clone valid", VolumeCloneRequest{Name: "c"}, ""},
		{"clone empty", VolumeCloneRequest{}, "name: cannot be blank."},
		{"resize valid", VolumeResizeRequest{Size: 1}, ""},
		{"resize empty", VolumeResizeRequest{}, "size: cannot be blank."},
		{"resize negative", VolumeResizeRequest{Size: -5}, "size: must be no less than 1."},
		{"rename valid", VolumeRenameRequest{Name: "r"}, ""},
		{"rename empty", VolumeRenameRequest{}, "name: cannot be blank."},
		{"attach valid", VolumeAttachRequest{InstanceID: "i"}, ""},
		{"attach empty", VolumeAttachRequest{}, "instance_id: cannot be blank."},
		{"detach valid", VolumeDetachRequest{InstanceID: "i"}, ""},
		{"detach empty", VolumeDetachRequest{}, "instance_id: cannot be blank."},
	})
}

func TestValidateErrors_PrivateNetworks(t *testing.T) {
	empty := ""
	name := "n"
	runValidateCases(t, []validateCase{
		{"create valid", CreatePrivateNetworkRequest{Name: "n", Mode: NetworkModeCustom}, ""},
		{"create empty", CreatePrivateNetworkRequest{}, "name: cannot be blank."},
		{"create mode invalid", CreatePrivateNetworkRequest{Name: "n", Mode: "manual"}, "mode: must be a valid value."},
		{"update nil name", UpdatePrivateNetworkRequest{}, ""},
		{"update name set", UpdatePrivateNetworkRequest{Name: &name}, ""},
		{"update name empty", UpdatePrivateNetworkRequest{Name: &empty}, "name: cannot be blank."},
		{"subnet valid", CreateSubnetRequest{LocationCode: "FIN-01", Name: "s", CIDR: "10.0.0.0/24"}, ""},
		{"subnet empty", CreateSubnetRequest{}, "cidr: cannot be blank; location_code: cannot be blank; name: cannot be blank."},
		{"subnet cidr invalid", CreateSubnetRequest{LocationCode: "FIN-01", Name: "s", CIDR: "10.0.0.0/33"}, `cidr: must be a valid IPv4 CIDR, got "10.0.0.0/33".`},
		{"subnet cidr ipv6", CreateSubnetRequest{LocationCode: "FIN-01", Name: "s", CIDR: "fd00::/64"}, `cidr: must be a valid IPv4 CIDR, got "fd00::/64".`},
		{"update subnet nil name", UpdateSubnetRequest{}, ""},
		{"update subnet name empty", UpdateSubnetRequest{Name: &empty}, "name: cannot be blank."},
		{"route valid", CreateRouteRequest{Destination: "0.0.0.0/0", InstanceID: "i"}, ""},
		{"route empty", CreateRouteRequest{}, "destination: cannot be blank; instance_id: cannot be blank."},
		{"route destination invalid", CreateRouteRequest{Destination: "nope", InstanceID: "i"}, `destination: must be a valid IPv4 CIDR, got "nope".`},
		{"ip allocation auto", IPAllocAuto, ""},
		{"ip allocation none", IPAllocNone, ""},
		{"ip allocation ipv4", IPAllocation("10.0.0.5"), ""},
		{"ip allocation empty", IPAllocation(""), `must be "auto", "none", or a valid IPv4 address, got ""`},
		{"ip allocation ipv6", IPAllocation("fd00::1"), `must be "auto", "none", or a valid IPv4 address, got "fd00::1"`},
	})
}

func TestValidateErrors_Instances(t *testing.T) {
	valid := CreateInstanceRequest{InstanceType: "1V100.6V", Image: "ubuntu", Hostname: "h"}
	with := func(mut func(*CreateInstanceRequest)) CreateInstanceRequest {
		r := valid
		mut(&r)
		return r
	}
	runValidateCases(t, []validateCase{
		{"create valid", valid, ""},
		{"create empty", CreateInstanceRequest{}, "hostname: cannot be blank; image: cannot be blank; instance_type: cannot be blank."},
		{"create contract valid", with(func(r *CreateInstanceRequest) { r.Contract = "SPOT" }), ""},
		{"create contract invalid", with(func(r *CreateInstanceRequest) { r.Contract = "FOREVER" }), "contract: must be a valid value."},
		{"create startup script valid", with(func(r *CreateInstanceRequest) { r.StartupScript = "#!/bin/bash\necho hi" }), ""},
		{"create startup script at max runes", with(func(r *CreateInstanceRequest) { r.StartupScript = "#!" + strings.Repeat("ä", maxStartupScriptRunes-2) }), ""},
		{"create startup script too long", with(func(r *CreateInstanceRequest) { r.StartupScript = "#!" + strings.Repeat("a", maxStartupScriptRunes-1) }), "startup_script: the length must be no more than 65536."},
		{"create startup script without shebang", with(func(r *CreateInstanceRequest) { r.StartupScript = "echo hi" }), `startup_script: must start with "#!" (for example, "#!/bin/bash").`},
		{"create startup script and id", with(func(r *CreateInstanceRequest) {
			id := "script-id"
			r.StartupScript, r.StartupScriptID = "#!/bin/sh", &id
		}), "startup_script and startup_script_id cannot both be set"},
		{"create os volume valid", with(func(r *CreateInstanceRequest) { r.OSVolume = &OSVolumeCreateRequest{Name: "os", Size: 50} }), ""},
		{"create os volume empty", with(func(r *CreateInstanceRequest) { r.OSVolume = &OSVolumeCreateRequest{} }), "os_volume: (name: cannot be blank; size: cannot be blank.)."},
		{"create os volume size negative", with(func(r *CreateInstanceRequest) { r.OSVolume = &OSVolumeCreateRequest{Name: "os", Size: -1} }), "os_volume: (size: must be no less than 1.)."},
		{"create os volume spot policy invalid", with(func(r *CreateInstanceRequest) {
			r.OSVolume = &OSVolumeCreateRequest{Name: "os", Size: 50, OnSpotDiscontinue: "explode"}
		}), "os_volume: (on_spot_discontinue: must be a valid value.)."},
		{"create volumes valid", with(func(r *CreateInstanceRequest) {
			r.Volumes = []VolumeCreateRequest{{Name: "v", Size: 1, Type: VolumeTypeHDD}}
		}), ""},
		{"create volume invalid", with(func(r *CreateInstanceRequest) { r.Volumes = []VolumeCreateRequest{{}} }), "volumes: (0: (name: cannot be blank; size: cannot be blank; type: cannot be blank.).)."},
		{"create too many tags", with(func(r *CreateInstanceRequest) { r.Tags = tagRequests(MaxTagsPerResource + 1) }), "tags: the length must be no more than 10."},
		{"create invalid tag", with(func(r *CreateInstanceRequest) { r.Tags = []TagRequest{{Key: strings.Repeat("k", 64)}} }), "tags: (0: (key: the length must be no more than 63.).)."},
		{"create field errors before network rules", CreateInstanceRequest{PrivateIP: new(IPAllocAuto)}, "hostname: cannot be blank; image: cannot be blank; instance_type: cannot be blank."},
		{"action valid", InstanceActionRequest{Action: ActionBoot, ID: []string{"i"}}, ""},
		{"action empty", InstanceActionRequest{}, "action: cannot be blank; id: cannot be blank."},
		{"action ids empty slice", InstanceActionRequest{Action: ActionBoot, ID: []string{}}, "id: cannot be blank."},
		{"action invalid", InstanceActionRequest{Action: "fly", ID: []string{"i"}}, "action: must be a valid value."},
	})
}

func TestValidateErrors_ServerlessJobs(t *testing.T) {
	valid := CreateJobDeploymentRequest{
		Name:       "job",
		Containers: []CreateDeploymentContainer{{Image: "img:1"}},
		Compute:    &ContainerCompute{Name: "H100", Size: 1},
		Scaling:    &JobScalingOptions{MaxReplicaCount: 1, DeadlineSeconds: 600},
	}
	with := func(mut func(*CreateJobDeploymentRequest)) CreateJobDeploymentRequest {
		r := valid
		mut(&r)
		return r
	}
	runValidateCases(t, []validateCase{
		{"create valid", valid, ""},
		{"create empty", CreateJobDeploymentRequest{}, "compute: cannot be blank; containers: cannot be blank; name: cannot be blank; scaling: cannot be blank."},
		{"create compute zero", with(func(r *CreateJobDeploymentRequest) { r.Compute = &ContainerCompute{} }), "compute: cannot be blank."},
		{"create compute too large", with(func(r *CreateJobDeploymentRequest) { r.Compute = &ContainerCompute{Name: "H100", Size: 129} }), "compute: (size: must be no greater than 128.)."},
		{"create container invalid", with(func(r *CreateJobDeploymentRequest) { r.Containers = []CreateDeploymentContainer{{}} }), "containers: (0: (image: cannot be blank.).)."},
		{"create scaling zero", with(func(r *CreateJobDeploymentRequest) { r.Scaling = &JobScalingOptions{} }), "scaling: cannot be blank."},
		{"create scaling invalid", with(func(r *CreateJobDeploymentRequest) { r.Scaling = &JobScalingOptions{MaxReplicaCount: 1001} }), "scaling: (max_replica_count: must be no greater than 1000.)."},
		{"scaling zero", JobScalingOptions{}, ""},
		{"scaling at limits", JobScalingOptions{MaxReplicaCount: 1000, QueueMessageTTLSeconds: 604800, DeadlineSeconds: 604800}, ""},
		{"scaling replicas negative", JobScalingOptions{MaxReplicaCount: -1}, "max_replica_count: must be no less than 0."},
		{"scaling ttl negative", JobScalingOptions{QueueMessageTTLSeconds: -1}, "queue_message_ttl_seconds: must be no less than 0."},
		{"scaling ttl too large", JobScalingOptions{QueueMessageTTLSeconds: 604801}, "queue_message_ttl_seconds: must be no greater than 604800."},
		{"scaling deadline too small", JobScalingOptions{DeadlineSeconds: 599}, "deadline_seconds: must be no less than 600."},
		{"scaling deadline negative", JobScalingOptions{DeadlineSeconds: -1}, "deadline_seconds: must be no less than 600."},
		{"scaling deadline too large", JobScalingOptions{DeadlineSeconds: 604801}, "deadline_seconds: must be no greater than 604800."},
	})
}

func TestValidateErrors_ContainerDeployments(t *testing.T) {
	valid := CreateDeploymentRequest{
		Name:       "dep",
		Compute:    ContainerCompute{Name: "H100", Size: 1},
		Containers: []CreateDeploymentContainer{{Image: "img:1", ExposedPort: 80}},
	}
	with := func(mut func(*CreateDeploymentRequest)) CreateDeploymentRequest {
		r := valid
		mut(&r)
		return r
	}
	intPtr := func(v int) *int { return &v }
	runValidateCases(t, []validateCase{
		{"compute valid", ContainerCompute{Name: "H100", Size: 128}, ""},
		{"compute empty", ContainerCompute{}, "name: cannot be blank; size: cannot be blank."},
		{"compute size negative", ContainerCompute{Name: "H100", Size: -1}, "size: must be no less than 1."},
		{"compute size too large", ContainerCompute{Name: "H100", Size: 129}, "size: must be no greater than 128."},
		{"container valid", CreateDeploymentContainer{Image: "img:1", ExposedPort: 65535}, ""},
		{"container empty", CreateDeploymentContainer{}, "image: cannot be blank."},
		{"container port too large", CreateDeploymentContainer{Image: "img:1", ExposedPort: 65536}, "exposed_port: must be no greater than 65535."},
		{"container healthcheck port too large", CreateDeploymentContainer{Image: "img:1", Healthcheck: &ContainerHealthcheck{Enabled: true, Port: 65536}}, "healthcheck: (port: must be no greater than 65535.)."},
		{"healthcheck disabled skips port", ContainerHealthcheck{Port: 65536}, ""},
		{"healthcheck enabled port too large", ContainerHealthcheck{Enabled: true, Port: 65536}, "port: must be no greater than 65535."},
		{"deployment valid", valid, ""},
		{"deployment empty", CreateDeploymentRequest{}, "compute: cannot be blank; containers: cannot be blank; name: cannot be blank."},
		{"deployment compute size blank", with(func(r *CreateDeploymentRequest) { r.Compute = ContainerCompute{Name: "H100"} }), "compute: (size: cannot be blank.)."},
		{"deployment container invalid", with(func(r *CreateDeploymentRequest) { r.Containers = []CreateDeploymentContainer{{ExposedPort: 80}} }), "containers: (0: (image: cannot be blank.).)."},
		{"deployment exposed port missing", with(func(r *CreateDeploymentRequest) { r.Containers = []CreateDeploymentContainer{{Image: "img:1"}} }), "containers[0].exposed_port is required and must be >= 1"},
		{"scaling update empty", UpdateScalingOptionsRequest{}, "at least one scaling option must be set"},
		{"scaling update valid", UpdateScalingOptionsRequest{MinReplicaCount: intPtr(0), MaxReplicaCount: intPtr(1000)}, ""},
		{"scaling update min negative", UpdateScalingOptionsRequest{MinReplicaCount: intPtr(-1)}, "min_replica_count: must be no less than 0."},
		{"scaling update max too large", UpdateScalingOptionsRequest{MaxReplicaCount: intPtr(1001)}, "max_replica_count: must be no greater than 1000."},
		{"scaling update min above max", UpdateScalingOptionsRequest{MinReplicaCount: intPtr(3), MaxReplicaCount: intPtr(2)}, "min_replica_count (3) must not be greater than max_replica_count (2)"},
		{"scaling update ttl too large", UpdateScalingOptionsRequest{QueueMessageTTLSeconds: intPtr(86401)}, "queue_message_ttl_seconds: must be no greater than 86400."},
		{"scaling update concurrency negative", UpdateScalingOptionsRequest{ConcurrentRequestsPerReplica: intPtr(-1)}, "concurrent_requests_per_replica: must be no less than 0."},
		{"scaling update concurrency too large", UpdateScalingOptionsRequest{ConcurrentRequestsPerReplica: intPtr(5001)}, "concurrent_requests_per_replica: must be no greater than 5000."},
		{"scaling update policy delay too large", UpdateScalingOptionsRequest{ScaleDownPolicy: &ScalingPolicy{DelaySeconds: 3601}}, "scale_down_policy: (delay_seconds: must be no greater than 3600.)."},
		{"scaling update up policy negative", UpdateScalingOptionsRequest{ScaleUpPolicy: &ScalingPolicy{DelaySeconds: -1}}, "scale_up_policy: (delay_seconds: must be no less than 0.)."},
		{"scaling update queue load blank", UpdateScalingOptionsRequest{ScalingTriggers: &ScalingTriggers{QueueLoad: &QueueLoadTrigger{}}}, "scaling_triggers: (queue_load: (threshold: cannot be blank.).)."},
		{"scaling update several errors", UpdateScalingOptionsRequest{MinReplicaCount: intPtr(-1), MaxReplicaCount: intPtr(1001)}, "max_replica_count: must be no greater than 1000; min_replica_count: must be no less than 0."},
		{"scaling options zero", ContainerScalingOptions{}, ""},
		{"scaling options max too large", ContainerScalingOptions{MaxReplicaCount: 1001}, "max_replica_count: must be no greater than 1000."},
		{"policy valid", ScalingPolicy{DelaySeconds: 3600}, ""},
		{"policy negative", ScalingPolicy{DelaySeconds: -1}, "delay_seconds: must be no less than 0."},
		{"triggers empty", ScalingTriggers{}, ""},
		{"triggers cpu below min", ScalingTriggers{CPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 4}}, "cpu_utilization: (threshold: must be no less than 5.)."},
		{"triggers gpu above max", ScalingTriggers{GPUUtilization: &UtilizationTrigger{Enabled: true, Threshold: 101}}, "gpu_utilization: (threshold: must be no greater than 100.)."},
		{"queue load valid", QueueLoadTrigger{Threshold: 1000}, ""},
		{"queue load blank", QueueLoadTrigger{}, "threshold: cannot be blank."},
		{"queue load below min", QueueLoadTrigger{Threshold: 0.5}, "threshold: must be no less than 1."},
		{"queue load above max", QueueLoadTrigger{Threshold: 1000.5}, "threshold: must be no greater than 1000."},
		{"queue load NaN", QueueLoadTrigger{Threshold: math.NaN()}, "threshold: must be no less than 1."},
		{"utilization disabled skips threshold", UtilizationTrigger{Threshold: 4}, ""},
		{"utilization enabled zero threshold", UtilizationTrigger{Enabled: true}, ""},
		{"utilization valid", UtilizationTrigger{Enabled: true, Threshold: 5}, ""},
		{"utilization below min", UtilizationTrigger{Enabled: true, Threshold: -1}, "threshold: must be no less than 5."},
		{"secret valid", CreateSecretRequest{Name: "s", Value: "v"}, ""},
		{"secret empty", CreateSecretRequest{}, "name: cannot be blank; value: cannot be blank."},
		{"file secret valid", CreateFileSecretRequest{Name: "s", Files: []FileSecretFile{{Name: "f"}}}, ""},
		{"file secret empty", CreateFileSecretRequest{}, "files: cannot be blank; name: cannot be blank."},
		{"file secret empty files", CreateFileSecretRequest{Name: "s", Files: []FileSecretFile{}}, "files: cannot be blank."},
		{"registry credentials valid", CreateRegistryCredentialsRequest{Name: "r", Type: "ghcr"}, ""},
		{"registry credentials empty", CreateRegistryCredentialsRequest{}, "name: cannot be blank; type: cannot be blank."},
		{"registry credentials type invalid", CreateRegistryCredentialsRequest{Name: "r", Type: "quay"}, "type: must be a valid value."},
	})
}

func TestValidateErrors_ManagedKubernetes(t *testing.T) {
	valid := CreateManagedKubernetesClusterRequest{
		Name: "c1", KubernetesVersion: "1.30", PodCIDR: "10.0.0.0/16", ServiceCIDR: "10.96.0.0/16",
		Location: "FIN-01", NetworkID: "n", SubnetID: "s",
	}
	with := func(mut func(*CreateManagedKubernetesClusterRequest)) CreateManagedKubernetesClusterRequest {
		r := valid
		mut(&r)
		return r
	}
	zero, negative := 0, -1
	runValidateCases(t, []validateCase{
		{"cluster valid", valid, ""},
		{"cluster empty", CreateManagedKubernetesClusterRequest{}, "kubernetes_version: cannot be blank; location: cannot be blank; name: cannot be blank; network: cannot be blank; pod_cidr: cannot be blank; service_cidr: cannot be blank; subnet: cannot be blank."},
		{"cluster name too long", with(func(r *CreateManagedKubernetesClusterRequest) { r.Name = strings.Repeat("a", 31) }), "name: the length must be between 1 and 30."},
		{"cluster name not dns label", with(func(r *CreateManagedKubernetesClusterRequest) { r.Name = "Bad_Name" }), "name: must be in a valid format."},
		{"cluster pod cidr invalid", with(func(r *CreateManagedKubernetesClusterRequest) { r.PodCIDR = "nope" }), `pod_cidr: must be a valid IPv4 CIDR, got "nope".`},
		{"cluster pod cidr too small", with(func(r *CreateManagedKubernetesClusterRequest) { r.PodCIDR = "10.0.0.0/24" }), `pod_cidr: must be /21 or larger, got "10.0.0.0/24".`},
		{"cluster service cidr invalid", with(func(r *CreateManagedKubernetesClusterRequest) { r.ServiceCIDR = "fd00::/108" }), `service_cidr: must be a valid IPv4 CIDR, got "fd00::/108".`},
		{"cluster cidrs overlap", with(func(r *CreateManagedKubernetesClusterRequest) { r.ServiceCIDR = "10.0.1.0/24" }), "service_cidr must not overlap pod_cidr"},
		{"cluster update valid", UpdateManagedKubernetesClusterRequest{KubernetesVersion: "1.31"}, ""},
		{"cluster update empty", UpdateManagedKubernetesClusterRequest{}, "kubernetes_version: cannot be blank."},
		{"pool valid", CreateManagedKubernetesNodePoolRequest{Name: "p1", InstanceType: "CPU.4V", SSHKeyIDs: []string{"k"}, Replicas: &zero}, ""},
		{"pool empty", CreateManagedKubernetesNodePoolRequest{}, "instance_type: cannot be blank; name: cannot be blank; ssh_key_ids: cannot be blank."},
		{"pool name too long", CreateManagedKubernetesNodePoolRequest{Name: strings.Repeat("a", 31), InstanceType: "CPU.4V", SSHKeyIDs: []string{"k"}}, "name: the length must be between 1 and 30."},
		{"pool name not dns label", CreateManagedKubernetesNodePoolRequest{Name: "-p", InstanceType: "CPU.4V", SSHKeyIDs: []string{"k"}}, "name: must be in a valid format."},
		{"pool replicas negative", CreateManagedKubernetesNodePoolRequest{Name: "p1", InstanceType: "CPU.4V", SSHKeyIDs: []string{"k"}, Replicas: &negative}, "replicas: must be no less than 0."},
		{"pool update zero", UpdateManagedKubernetesNodePoolRequest{Replicas: &zero}, ""},
		{"pool update nil", UpdateManagedKubernetesNodePoolRequest{}, "replicas: cannot be blank."},
		{"pool update negative", UpdateManagedKubernetesNodePoolRequest{Replicas: &negative}, "replicas: must be no less than 0."},
		{"kubeconfig valid", CreateManagedKubernetesKubeconfigRequest{CommonName: "alice", Groups: []string{"dev", "system:masters"}, DisplayName: "Alice", Duration: "720h"}, ""},
		{"kubeconfig empty", CreateManagedKubernetesKubeconfigRequest{}, "common_name: cannot be blank."},
		{"kubeconfig common name system", CreateManagedKubernetesKubeconfigRequest{CommonName: "system:admin"}, `common_name: "system:admin" is a control-plane identity.`},
		{"kubeconfig group empty", CreateManagedKubernetesKubeconfigRequest{CommonName: "a", Groups: []string{"ok", " "}}, "groups: (1: must not be empty.)."},
		{"kubeconfig group system", CreateManagedKubernetesKubeconfigRequest{CommonName: "a", Groups: []string{"system:nodes"}}, `groups: (0: "system:nodes" is a control-plane identity.).`},
		{"kubeconfig display name at max runes", CreateManagedKubernetesKubeconfigRequest{CommonName: "a", DisplayName: strings.Repeat("ä", 128)}, ""},
		{"kubeconfig display name too long", CreateManagedKubernetesKubeconfigRequest{CommonName: "a", DisplayName: strings.Repeat("ä", 129)}, "display_name: the length must be no more than 128."},
		{"kubeconfig display name not printable", CreateManagedKubernetesKubeconfigRequest{CommonName: "a", DisplayName: "a\x00"}, "display_name: must contain printable characters only."},
		{"kubeconfig duration invalid", CreateManagedKubernetesKubeconfigRequest{CommonName: "a", Duration: "x"}, `duration: invalid duration "x".`},
		{"kubeconfig duration too short", CreateManagedKubernetesKubeconfigRequest{CommonName: "a", Duration: "30m"}, `duration: duration must be at least 1h, got "30m".`},
	})
}
