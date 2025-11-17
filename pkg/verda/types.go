package verda

import "time"

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
	PricePerHour    float64         `json:"price_per_hour"`
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

// OSVolumeCreateRequest represents OS volume configuration
type OSVolumeCreateRequest struct {
	Name string `json:"name"`
	Size int    `json:"size"`
	Type string `json:"type"`
}

// InstanceActionRequest represents an action to perform on instances
type InstanceActionRequest struct {
	IDList    any      `json:"id_list"` // Can be string or []string
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

// Container represents a container instance
type Container struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Status      string            `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	Environment map[string]string `json:"environment,omitempty"`
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
