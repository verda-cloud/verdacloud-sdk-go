package verda

import (
	"encoding/json"
	"strconv"
	"time"
)

// InstanceCPU represents CPU information
type InstanceCPU struct {
	Description   string `json:"description"`
	NumberOfCores int    `json:"number_of_cores"`
}

// InstanceGPU represents GPU information
type InstanceGPU struct {
	Description  string `json:"description"`
	NumberOfGPUs int    `json:"number_of_gpus"`
}

// InstanceMemory represents memory information
type InstanceMemory struct {
	Description     string `json:"description"`
	SizeInGigabytes int    `json:"size_in_gigabytes"`
}

// InstanceStorage represents storage information
type InstanceStorage struct {
	Description string `json:"description"`
}

// Instance represents a Verda instance
type Instance struct {
	ID              string          `json:"id"`
	InstanceType    string          `json:"instance_type"`
	Image           string          `json:"image"`
	PricePerHour    FlexibleFloat   `json:"price_per_hour"`
	Hostname        string          `json:"hostname"`
	Description     string          `json:"description"`
	IP              *string         `json:"ip"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	SSHKeyIDs       []string        `json:"ssh_key_ids"`
	CPU             InstanceCPU     `json:"cpu"`
	GPU             InstanceGPU     `json:"gpu"`
	Memory          InstanceMemory  `json:"memory"`
	Storage         InstanceStorage `json:"storage"`
	OSVolumeID      *string         `json:"os_volume_id"`
	GPUMemory       InstanceMemory  `json:"gpu_memory"`
	Location        string          `json:"location"`
	IsSpot          bool            `json:"is_spot"`
	OSName          string          `json:"os_name"`
	StartupScriptID *string         `json:"startup_script_id"`
	JupyterToken    *string         `json:"jupyter_token"`
	Contract        string          `json:"contract"`
	Pricing         string          `json:"pricing"`
}

// CreateInstanceRequest represents the request to create an instance
type CreateInstanceRequest struct {
	InstanceType    string                 `json:"instance_type"`
	Image           string                 `json:"image"`
	Hostname        string                 `json:"hostname"`
	Description     string                 `json:"description"`
	SSHKeyIDs       []string               `json:"ssh_key_ids,omitempty"`
	LocationCode    string                 `json:"location_code,omitempty"`
	Contract        string                 `json:"contract,omitempty"`
	Pricing         string                 `json:"pricing,omitempty"`
	StartupScriptID *string                `json:"startup_script_id,omitempty"`
	Volumes         []VolumeCreateRequest  `json:"volumes,omitempty"`
	ExistingVolumes []string               `json:"existing_volumes,omitempty"`
	OSVolume        *OSVolumeCreateRequest `json:"os_volume,omitempty"`
	IsSpot          bool                   `json:"is_spot,omitempty"`
	Coupon          *string                `json:"coupon,omitempty"`
}

// VolumeCreateRequest represents a volume to be created
type VolumeCreateRequest struct {
	Size     int    `json:"size"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Location string `json:"location,omitempty"`
}

// VolumeAttachRequest represents a request to attach a volume to an instance
type VolumeAttachRequest struct {
	InstanceID string `json:"instance_id"`
}

// VolumeDetachRequest represents a request to detach a volume from an instance
type VolumeDetachRequest struct {
	InstanceID string `json:"instance_id"`
}

// VolumeCloneRequest represents a request to clone a volume
type VolumeCloneRequest struct {
	Name     string `json:"name"`
	Location string `json:"location,omitempty"`
}

// VolumeResizeRequest represents a request to resize a volume
type VolumeResizeRequest struct {
	Size int `json:"size"`
}

// VolumeRenameRequest represents a request to rename a volume
type VolumeRenameRequest struct {
	Name string `json:"name"`
}

// OSVolumeCreateRequest represents OS volume configuration
type OSVolumeCreateRequest struct {
	Name string `json:"name"`
	Size int    `json:"size"`
	Type string `json:"type"`
}

// InstanceActionRequest represents an action to perform on instances
type InstanceActionRequest struct {
	ID        string   `json:"id_list"`
	Action    string   `json:"action"`
	VolumeIDs []string `json:"volume_ids,omitempty"`
}

// InstanceAvailability represents instance availability information
type InstanceAvailability struct {
	InstanceType string `json:"instance_type"`
	Location     string `json:"location"`
	Available    bool   `json:"available"`
	IsSpot       bool   `json:"is_spot"`
}

// LocationAvailability represents instance type availability by location
type LocationAvailability struct {
	LocationCode   string   `json:"location_code"`
	Availabilities []string `json:"availabilities"`
}

// Volume represents a Verda volume
type Volume struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Size       int       `json:"size"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	InstanceID *string   `json:"instance_id"`
}

// VolumeType represents available volume type specifications
type VolumeType struct {
	Type                 string          `json:"type"`
	Price                VolumeTypePrice `json:"price"`
	IsSharedFS           bool            `json:"is_shared_fs"`
	BurstBandwidth       float64         `json:"burst_bandwidth"`
	ContinuousBandwidth  float64         `json:"continuous_bandwidth"`
	InternalNetworkSpeed float64         `json:"internal_network_speed"`
	IOPS                 string          `json:"iops"`
}

// VolumeTypePrice represents the pricing structure for a volume type
type VolumeTypePrice struct {
	MonthlyPerGB float64 `json:"monthly_per_gb"`
	Currency     string  `json:"currency"`
}

// SSHKey represents an SSH key
type SSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"key"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

// StartupScript represents a startup script
type StartupScript struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Script    string    `json:"script"`
	CreatedAt time.Time `json:"created_at"`
}

// Location represents a datacenter location
type Location struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Available   bool   `json:"available"`
}

// Balance represents account balance information
type Balance struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// Image represents an OS image for instances
type Image struct {
	ID        string   `json:"id"`
	ImageType string   `json:"image_type"`
	Name      string   `json:"name"`
	IsDefault bool     `json:"is_default"`
	IsCluster bool     `json:"is_cluster"`
	Details   []string `json:"details"`
	Category  string   `json:"category"`
}

// ContainerType represents a serverless container compute resource option
type ContainerType struct {
	ID                  string         `json:"id"`
	Model               string         `json:"model"`
	Name                string         `json:"name"`
	InstanceType        string         `json:"instance_type"`
	CPU                 InstanceCPU    `json:"cpu"`
	GPU                 InstanceGPU    `json:"gpu"`
	GPUMemory           InstanceMemory `json:"gpu_memory"`
	Memory              InstanceMemory `json:"memory"`
	ServerlessPrice     FlexibleFloat  `json:"serverless_price"`
	ServerlessSpotPrice FlexibleFloat  `json:"serverless_spot_price"`
	Currency            string         `json:"currency"`
	Manufacturer        string         `json:"manufacturer"`
}

// InstanceTypeInfo represents detailed instance type information with pricing
type InstanceTypeInfo struct {
	ID              string          `json:"id"`
	InstanceType    string          `json:"instance_type"`
	Model           string          `json:"model"`
	Name            string          `json:"name"`
	CPU             InstanceCPU     `json:"cpu"`
	GPU             InstanceGPU     `json:"gpu"`
	GPUMemory       InstanceMemory  `json:"gpu_memory"`
	Memory          InstanceMemory  `json:"memory"`
	PricePerHour    FlexibleFloat   `json:"price_per_hour"`
	SpotPrice       FlexibleFloat   `json:"spot_price"`
	DynamicPrice    FlexibleFloat   `json:"dynamic_price"`
	MaxDynamicPrice FlexibleFloat   `json:"max_dynamic_price"`
	Storage         InstanceStorage `json:"storage"`
	Currency        string          `json:"currency"`
	Manufacturer    string          `json:"manufacturer"`
	BestFor         []string        `json:"best_for"`
	Description     string          `json:"description"`
}

// PriceHistoryRecord represents a single price record in the price history
type PriceHistoryRecord struct {
	Date                string        `json:"date"`
	FixedPricePerHour   FlexibleFloat `json:"fixed_price_per_hour"`
	DynamicPricePerHour FlexibleFloat `json:"dynamic_price_per_hour"`
	Currency            string        `json:"currency"`
}

// InstanceTypePriceHistory maps instance type names to their price history records
type InstanceTypePriceHistory map[string][]PriceHistoryRecord

// LongTermPeriod represents a long-term rental period option
type LongTermPeriod struct {
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	IsEnabled          bool    `json:"is_enabled"`
	UnitName           string  `json:"unit_name"`
	UnitValue          int     `json:"unit_value"`
	DiscountPercentage float64 `json:"discount_percentage"`
}

// Action constants
const (
	ActionBoot          = "boot"
	ActionStart         = "start"
	ActionShutdown      = "shutdown"
	ActionDelete        = "delete"
	ActionDiscontinue   = "discontinue"
	ActionHibernate     = "hibernate"
	ActionConfigureSpot = "configure_spot"
	ActionForceShutdown = "force_shutdown"
	ActionDeleteStuck   = "delete_stuck"
	ActionDeploy        = "deploy"
	ActionTransfer      = "transfer"
)

// Instance status constants
const (
	StatusRunning = "RUNNING"
	StatusOffline = "OFFLINE"
	StatusPending = "PENDING"
)

// Default location
const (
	LocationFIN01 = "FIN-01"
)

// Volume type constants
const (
	VolumeTypeHDD               = "HDD"
	VolumeTypeNVMe              = "NVMe"
	VolumeTypeHDDShared         = "HDD_Shared"
	VolumeTypeNVMeShared        = "NVMe_Shared"
	VolumeTypeNVMeLocalStorage  = "NVMe_Local_Storage"
	VolumeTypeNVMeSharedCluster = "NVMe_Shared_Cluster"
	VolumeTypeNVMeOSCluster     = "NVMe_OS_Cluster"
)

// Volume status constants - these match the actual API values
const (
	VolumeStatusOrdered   = "ordered"
	VolumeStatusAttached  = "attached"
	VolumeStatusAttaching = "attaching"
	VolumeStatusDetached  = "detached"
	VolumeStatusDeleted   = "deleted"
	VolumeStatusCloning   = "cloning"
	VolumeStatusDetaching = "detaching"
	VolumeStatusDeleting  = "deleting"
	VolumeStatusRestoring = "restoring"
	VolumeStatusCreated   = "created"
	VolumeStatusExported  = "exported"
	VolumeStatusCanceled  = "canceled"
	VolumeStatusCanceling = "canceling"
)

// Cluster represents a Verda cluster
type Cluster struct {
	ID              string          `json:"id"`
	ClusterType     string          `json:"cluster_type"`
	Image           string          `json:"image"`
	PricePerHour    FlexibleFloat   `json:"price_per_hour"`
	Hostname        string          `json:"hostname"`
	Description     string          `json:"description"`
	IP              *string         `json:"ip"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	SSHKeyIDs       []string        `json:"ssh_key_ids"`
	CPU             InstanceCPU     `json:"cpu"`
	GPU             InstanceGPU     `json:"gpu"`
	Memory          InstanceMemory  `json:"memory"`
	Storage         InstanceStorage `json:"storage"`
	GPUMemory       InstanceMemory  `json:"gpu_memory"`
	Location        string          `json:"location"`
	OSName          string          `json:"os_name"`
	StartupScriptID *string         `json:"startup_script_id"`
	Contract        string          `json:"contract"`
	Pricing         string          `json:"pricing"`
}

// CreateClusterRequest represents the request to create a cluster
type CreateClusterRequest struct {
	ClusterType     string   `json:"cluster_type"`
	Image           string   `json:"image"`
	Hostname        string   `json:"hostname"`
	Description     string   `json:"description,omitempty"`
	SSHKeyIDs       []string `json:"ssh_key_ids"`
	LocationCode    string   `json:"location_code,omitempty"`
	Contract        string   `json:"contract,omitempty"`
	Pricing         string   `json:"pricing,omitempty"`
	StartupScriptID *string  `json:"startup_script_id,omitempty"`
	SharedVolumes   []string `json:"shared_volumes,omitempty"`
	ExistingVolumes []string `json:"existing_volumes,omitempty"`
	Coupon          *string  `json:"coupon,omitempty"`
}

// CreateClusterResponse represents the response from creating a cluster
type CreateClusterResponse struct {
	ID string `json:"id"`
}

// ClusterActionRequest represents an action to perform on clusters
type ClusterActionRequest struct {
	IDList any    `json:"id_list"` // Can be string or []string
	Action string `json:"action"`
}

// ClusterAvailability represents cluster availability information
type ClusterAvailability struct {
	ClusterType  string `json:"cluster_type"`
	LocationCode string `json:"location_code"`
	Available    bool   `json:"available"`
}

// ClusterType represents a cluster configuration type
type ClusterType struct {
	ClusterType  string          `json:"cluster_type"`
	Description  string          `json:"description"`
	PricePerHour FlexibleFloat   `json:"price_per_hour"`
	CPU          InstanceCPU     `json:"cpu"`
	GPU          InstanceGPU     `json:"gpu"`
	Memory       InstanceMemory  `json:"memory"`
	Storage      InstanceStorage `json:"storage"`
	GPUMemory    InstanceMemory  `json:"gpu_memory"`
	Manufacturer string          `json:"manufacturer"`
	Available    bool            `json:"available"`
}

// ClusterImage represents an OS image for clusters
type ClusterImage struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Available   bool   `json:"available"`
}

// Cluster action constants
const (
	ClusterActionDiscontinue = "discontinue"
)

// Container Deployment Types

// ContainerDeployment represents a serverless container deployment
type ContainerDeployment struct {
	Name        string                 `json:"name"`
	Image       string                 `json:"image"`
	Status      string                 `json:"status"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
	Replicas    int                    `json:"replicas"`
	Environment map[string]string      `json:"environment,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// CreateDeploymentRequest represents a request to create a new deployment
type CreateDeploymentRequest struct {
	Name        string                 `json:"name"`
	Image       string                 `json:"image"`
	Replicas    int                    `json:"replicas,omitempty"`
	Environment map[string]string      `json:"environment,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// UpdateDeploymentRequest represents a request to update a deployment
type UpdateDeploymentRequest struct {
	Image       string                 `json:"image,omitempty"`
	Replicas    int                    `json:"replicas,omitempty"`
	Environment map[string]string      `json:"environment,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// DeploymentStatus represents the status of a deployment
type DeploymentStatus struct {
	Status            string `json:"status"`
	DesiredReplicas   int    `json:"desired_replicas"`
	CurrentReplicas   int    `json:"current_replicas"`
	AvailableReplicas int    `json:"available_replicas"`
	UpdatedAt         string `json:"updated_at"`
}

// ScalingOptions represents scaling configuration for a deployment
type ScalingOptions struct {
	MinReplicas int `json:"min_replicas"`
	MaxReplicas int `json:"max_replicas"`
	TargetCPU   int `json:"target_cpu_percent,omitempty"`
}

// UpdateScalingOptionsRequest represents a request to update scaling options
type UpdateScalingOptionsRequest struct {
	MinReplicas *int `json:"min_replicas,omitempty"`
	MaxReplicas *int `json:"max_replicas,omitempty"`
	TargetCPU   *int `json:"target_cpu_percent,omitempty"`
}

// DeploymentReplicas represents replica information for a deployment
type DeploymentReplicas struct {
	Replicas []ReplicaInfo `json:"replicas"`
}

// ReplicaInfo represents information about a single replica
type ReplicaInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Node   string `json:"node,omitempty"`
}

// EnvironmentVariablesRequest represents a request to add/update environment variables
type EnvironmentVariablesRequest struct {
	Variables map[string]string `json:"variables"`
}

// DeleteEnvironmentVariablesRequest represents a request to delete environment variables
type DeleteEnvironmentVariablesRequest struct {
	Names []string `json:"names"`
}

// ComputeResource represents available compute resources
type ComputeResource struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Available bool   `json:"available"`
	CPU       string `json:"cpu,omitempty"`
	Memory    string `json:"memory,omitempty"`
	GPU       string `json:"gpu,omitempty"`
}

// Secret represents a secret used in deployments
type Secret struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateSecretRequest represents a request to create a new secret
type CreateSecretRequest struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

// FileSecret represents a fileset secret
type FileSecret struct {
	Name      string   `json:"name"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Files     []string `json:"files,omitempty"`
}

// CreateFileSecretRequest represents a request to create a fileset secret
type CreateFileSecretRequest struct {
	Name  string            `json:"name"`
	Files map[string]string `json:"files"`
}

// RegistryCredentials represents container registry credentials
type RegistryCredentials struct {
	Name      string `json:"name"`
	Registry  string `json:"registry"`
	CreatedAt string `json:"created_at"`
}

// CreateRegistryCredentialsRequest represents a request to create registry credentials
type CreateRegistryCredentialsRequest struct {
	Name     string `json:"name"`
	Registry string `json:"registry"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Serverless Jobs Types

// JobDeploymentShortInfo represents summary information about a job deployment
type JobDeploymentShortInfo struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	Compute   string `json:"compute,omitempty"`
}

// JobDeployment represents a complete serverless job deployment
type JobDeployment struct {
	Name       string                 `json:"name"`
	Status     string                 `json:"status"`
	CreatedAt  string                 `json:"created_at"`
	UpdatedAt  string                 `json:"updated_at"`
	Containers []JobContainer         `json:"containers,omitempty"`
	Scaling    *JobScalingOptions     `json:"scaling,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
}

// JobContainer represents a container configuration in a job deployment
type JobContainer struct {
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Command []string          `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// CreateJobDeploymentRequest represents a request to create a new job deployment
type CreateJobDeploymentRequest struct {
	Name       string                 `json:"name"`
	Containers []JobContainer         `json:"containers"`
	Scaling    *JobScalingOptions     `json:"scaling,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
}

// JobScalingOptions represents scaling configuration for a job deployment
type JobScalingOptions struct {
	MinReplicas           int `json:"min_replicas"`
	MaxReplicas           int `json:"max_replicas"`
	PollingInterval       int `json:"polling_interval,omitempty"`
	CooldownPeriod        int `json:"cooldown_period,omitempty"`
	MaxReplicaCount       int `json:"max_replica_count,omitempty"`
	SuccessfulJobsHistory int `json:"successful_jobs_history,omitempty"`
	FailedJobsHistory     int `json:"failed_jobs_history,omitempty"`
}

// JobDeploymentStatus represents the status of a job deployment
type JobDeploymentStatus struct {
	Status        string `json:"status"`
	ActiveJobs    int    `json:"active_jobs,omitempty"`
	SucceededJobs int    `json:"succeeded_jobs,omitempty"`
	FailedJobs    int    `json:"failed_jobs,omitempty"`
}

// FlexibleFloat is a custom type that can unmarshal both string and float64 values
type FlexibleFloat float64

// UnmarshalJSON implements json.Unmarshaler to handle both string and float64 inputs
func (f *FlexibleFloat) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as float64 first
	var floatVal float64
	if err := json.Unmarshal(data, &floatVal); err == nil {
		*f = FlexibleFloat(floatVal)
		return nil
	}

	// Try to unmarshal as string
	var strVal string
	if err := json.Unmarshal(data, &strVal); err != nil {
		return err
	}

	// Convert string to float64
	floatVal, err := strconv.ParseFloat(strVal, 64)
	if err != nil {
		return err
	}

	*f = FlexibleFloat(floatVal)
	return nil
}

// MarshalJSON implements json.Marshaler to always marshal as float64
func (f FlexibleFloat) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(f))
}

// Float64 returns the float64 value
func (f FlexibleFloat) Float64() float64 {
	return float64(f)
}
