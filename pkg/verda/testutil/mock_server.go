package testutil

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// API path constants
const (
	pathSSHKeys    = "/sshkeys"
	pathSSHKeysAlt = "/ssh-keys"
	pathScripts    = "/scripts"
	pathClusters   = "/clusters"
)

// TestClientConfig holds configuration for creating test clients
type TestClientConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
}

// NewTestClientConfig creates a standard test client configuration for the given mock server
func NewTestClientConfig(mockServer *MockServer) TestClientConfig {
	return TestClientConfig{
		BaseURL:      mockServer.URL(),
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
	}
}

// Mock types to avoid circular imports
type Instance struct {
	ID              string                 `json:"id"`
	InstanceType    string                 `json:"instance_type"`
	Image           string                 `json:"image"`
	PricePerHour    float64                `json:"price_per_hour"`
	Hostname        string                 `json:"hostname"`
	Description     string                 `json:"description"`
	IP              *string                `json:"ip"`
	Status          string                 `json:"status"`
	CreatedAt       time.Time              `json:"created_at"`
	SSHKeyIDs       []string               `json:"ssh_key_ids"`
	CPU             map[string]interface{} `json:"cpu"`
	GPU             map[string]interface{} `json:"gpu"`
	Memory          map[string]interface{} `json:"memory"`
	Storage         map[string]interface{} `json:"storage"`
	OSVolumeID      *string                `json:"os_volume_id"`
	GPUMemory       map[string]interface{} `json:"gpu_memory"`
	Location        string                 `json:"location"`
	IsSpot          bool                   `json:"is_spot"`
	OSName          string                 `json:"os_name"`
	StartupScriptID *string                `json:"startup_script_id"`
	JupyterToken    *string                `json:"jupyter_token"`
	Contract        string                 `json:"contract"`
	Pricing         string                 `json:"pricing"`
}

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

type VolumeCreateRequest struct {
	Size int    `json:"size"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type OSVolumeCreateRequest struct {
	Size int    `json:"size"`
	Type string `json:"type"`
}

type InstanceActionRequest struct {
	ID        string   `json:"id_list"`
	Action    string   `json:"action"`
	VolumeIDs []string `json:"volume_ids,omitempty"`
}

type Balance struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type SSHKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"key"`
	Fingerprint string    `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

type Location struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Available   bool   `json:"available"`
}

type Volume struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Size       int       `json:"size"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	InstanceID *string   `json:"instance_id"`
}

type StartupScript struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Script    string    `json:"script"`
	CreatedAt time.Time `json:"created_at"`
}

type VolumeType struct {
	Type                 string          `json:"type"`
	Price                VolumeTypePrice `json:"price"`
	IsSharedFS           bool            `json:"is_shared_fs"`
	BurstBandwidth       float64         `json:"burst_bandwidth"`
	ContinuousBandwidth  float64         `json:"continuous_bandwidth"`
	InternalNetworkSpeed float64         `json:"internal_network_speed"`
	IOPS                 string          `json:"iops"`
}

type VolumeTypePrice struct {
	MonthlyPerGB float64 `json:"monthly_per_gb"`
	Currency     string  `json:"currency"`
}

type Image struct {
	ID        string   `json:"id"`
	ImageType string   `json:"image_type"`
	Name      string   `json:"name"`
	IsDefault bool     `json:"is_default"`
	IsCluster bool     `json:"is_cluster"`
	Details   []string `json:"details"`
	Category  string   `json:"category"`
}

type ContainerType struct {
	ID                  string                 `json:"id"`
	Model               string                 `json:"model"`
	Name                string                 `json:"name"`
	InstanceType        string                 `json:"instance_type"`
	CPU                 map[string]interface{} `json:"cpu"`
	GPU                 map[string]interface{} `json:"gpu"`
	GPUMemory           map[string]interface{} `json:"gpu_memory"`
	Memory              map[string]interface{} `json:"memory"`
	ServerlessPrice     float64                `json:"serverless_price"`
	ServerlessSpotPrice float64                `json:"serverless_spot_price"`
	Currency            string                 `json:"currency"`
	Manufacturer        string                 `json:"manufacturer"`
}

type InstanceTypeInfo struct {
	ID              string                 `json:"id"`
	InstanceType    string                 `json:"instance_type"`
	Model           string                 `json:"model"`
	Name            string                 `json:"name"`
	CPU             map[string]interface{} `json:"cpu"`
	GPU             map[string]interface{} `json:"gpu"`
	GPUMemory       map[string]interface{} `json:"gpu_memory"`
	Memory          map[string]interface{} `json:"memory"`
	PricePerHour    float64                `json:"price_per_hour"`
	SpotPrice       float64                `json:"spot_price"`
	DynamicPrice    float64                `json:"dynamic_price"`
	MaxDynamicPrice float64                `json:"max_dynamic_price"`
	Storage         map[string]interface{} `json:"storage"`
	Currency        string                 `json:"currency"`
	Manufacturer    string                 `json:"manufacturer"`
	BestFor         []string               `json:"best_for"`
	Description     string                 `json:"description"`
}

type LocationAvailability struct {
	LocationCode   string   `json:"location_code"`
	Availabilities []string `json:"availabilities"`
}

type PriceHistoryRecord struct {
	Date                string  `json:"date"`
	FixedPricePerHour   float64 `json:"fixed_price_per_hour"`
	DynamicPricePerHour float64 `json:"dynamic_price_per_hour"`
	Currency            string  `json:"currency"`
}

type LongTermPeriod struct {
	Code               string  `json:"code"`
	Name               string  `json:"name"`
	IsEnabled          bool    `json:"is_enabled"`
	UnitName           string  `json:"unit_name"`
	UnitValue          int     `json:"unit_value"`
	DiscountPercentage float64 `json:"discount_percentage"`
}

type Cluster struct {
	ID              string                 `json:"id"`
	ClusterType     string                 `json:"cluster_type"`
	Image           string                 `json:"image"`
	PricePerHour    float64                `json:"price_per_hour"`
	Hostname        string                 `json:"hostname"`
	Description     string                 `json:"description"`
	IP              *string                `json:"ip"`
	Status          string                 `json:"status"`
	CreatedAt       time.Time              `json:"created_at"`
	SSHKeyIDs       []string               `json:"ssh_key_ids"`
	CPU             map[string]interface{} `json:"cpu"`
	GPU             map[string]interface{} `json:"gpu"`
	Memory          map[string]interface{} `json:"memory"`
	Storage         map[string]interface{} `json:"storage"`
	GPUMemory       map[string]interface{} `json:"gpu_memory"`
	Location        string                 `json:"location"`
	OSName          string                 `json:"os_name"`
	StartupScriptID *string                `json:"startup_script_id"`
	Contract        string                 `json:"contract"`
	Pricing         string                 `json:"pricing"`
}

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

type ClusterActionRequest struct {
	IDList any    `json:"id_list"`
	Action string `json:"action"`
}

type ClusterType struct {
	ClusterType  string                 `json:"cluster_type"`
	Description  string                 `json:"description"`
	PricePerHour float64                `json:"price_per_hour"`
	CPU          map[string]interface{} `json:"cpu"`
	GPU          map[string]interface{} `json:"gpu"`
	Memory       map[string]interface{} `json:"memory"`
	Storage      map[string]interface{} `json:"storage"`
	GPUMemory    map[string]interface{} `json:"gpu_memory"`
	Manufacturer string                 `json:"manufacturer"`
	Available    bool                   `json:"available"`
}

type ClusterAvailability struct {
	ClusterType  string `json:"cluster_type"`
	LocationCode string `json:"location_code"`
	Available    bool   `json:"available"`
}

type ClusterImage struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Available   bool   `json:"available"`
}

const (
	StatusRunning = "RUNNING"
	StatusPending = "PENDING"
	LocationFIN01 = "FIN-01"
	pathInstances = "/instances"
	// nolint:gosec // G101: This is a URL path, not a credential
	pathOAuth2Token = "/oauth2/token"
)

// MockServer provides a test HTTP server for mocking Verda API responses
type MockServer struct {
	server   *httptest.Server
	handlers map[string]http.HandlerFunc
	mu       sync.RWMutex
	sshKeys  map[string]SSHKey        // Store created SSH keys
	scripts  map[string]StartupScript // Store created startup scripts
}

// NewMockServer creates a new mock server
func NewMockServer() *MockServer {
	ms := &MockServer{
		handlers: make(map[string]http.HandlerFunc),
		sshKeys:  make(map[string]SSHKey),
		scripts:  make(map[string]StartupScript),
	}

	ms.server = httptest.NewServer(http.HandlerFunc(ms.handleRequest))
	return ms
}

// URL returns the mock server URL
func (ms *MockServer) URL() string {
	return ms.server.URL
}

// Close shuts down the mock server
func (ms *MockServer) Close() {
	ms.server.Close()
}

// SetHandler sets a custom handler for a specific path and method
func (ms *MockServer) SetHandler(method, path string, handler http.HandlerFunc) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	key := method + " " + path
	ms.handlers[key] = handler
}

// handleRequest routes requests to appropriate handlers
//
//nolint:gocyclo // Mock server router naturally has high complexity
func (ms *MockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	ms.mu.RLock()
	key := r.Method + " " + r.URL.Path
	handler, exists := ms.handlers[key]
	ms.mu.RUnlock()

	if exists {
		handler(w, r)
		return
	}

	// Default handlers
	switch {
	case r.Method == http.MethodPost && r.URL.Path == pathOAuth2Token:
		ms.handleAuth(w, r)
	case r.Method == http.MethodGet && r.URL.Path == pathInstances:
		ms.handleGetInstances(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, pathInstances+"/"):
		ms.handleGetInstance(w, r)
	case r.Method == http.MethodPost && r.URL.Path == pathInstances:
		ms.handleCreateInstance(w, r)
	case r.Method == http.MethodPut && r.URL.Path == pathInstances:
		ms.handleInstanceAction(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/balance":
		ms.handleGetBalance(w, r)
	// SSH Keys - support both /ssh-keys and deprecated /sshkeys paths
	case r.Method == http.MethodGet && (r.URL.Path == pathSSHKeysAlt || r.URL.Path == pathSSHKeys):
		ms.handleGetSSHKeys(w, r)
	case r.Method == http.MethodGet && (strings.HasPrefix(r.URL.Path, pathSSHKeysAlt+"/") || strings.HasPrefix(r.URL.Path, pathSSHKeys+"/")):
		ms.handleGetSSHKey(w, r)
	case r.Method == http.MethodPost && (r.URL.Path == pathSSHKeysAlt || r.URL.Path == pathSSHKeys):
		ms.handleCreateSSHKey(w, r)
	case r.Method == http.MethodDelete && (strings.HasPrefix(r.URL.Path, pathSSHKeysAlt+"/") || strings.HasPrefix(r.URL.Path, pathSSHKeys+"/")):
		ms.handleDeleteSSHKey(w, r)
	case r.Method == http.MethodDelete && (r.URL.Path == pathSSHKeysAlt || r.URL.Path == pathSSHKeys):
		ms.handleDeleteMultipleSSHKeys(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/locations":
		ms.handleGetLocations(w, r)
	case r.Method == http.MethodGet && r.URL.Path == pathScripts:
		ms.handleGetScripts(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, pathScripts+"/"):
		ms.handleGetScript(w, r)
	case r.Method == http.MethodPost && r.URL.Path == pathScripts:
		ms.handleCreateScript(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, pathScripts+"/"):
		ms.handleDeleteScript(w, r)
	case r.Method == http.MethodDelete && r.URL.Path == pathScripts:
		ms.handleDeleteMultipleScripts(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/volume-types":
		ms.handleGetVolumeTypes(w, r)
	case r.Method == http.MethodGet && r.URL.Path == pathClusters:
		ms.handleGetClusters(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, pathClusters+"/"):
		ms.handleGetCluster(w, r)
	case r.Method == http.MethodPost && r.URL.Path == pathClusters:
		ms.handleCreateCluster(w, r)
	case r.Method == http.MethodPut && r.URL.Path == pathClusters:
		ms.handleClusterAction(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/cluster-types":
		ms.handleGetClusterTypes(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/cluster-availability":
		ms.handleGetClusterAvailabilities(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/cluster-availability/"):
		ms.handleCheckClusterTypeAvailability(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/images/cluster":
		ms.handleGetClusterImages(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/images":
		ms.handleGetImages(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/container-types":
		ms.handleGetContainerTypes(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/instance-types":
		ms.handleGetInstanceTypes(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/instance-types/price-history":
		ms.handleGetPriceHistory(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/instance-types/"):
		ms.handleGetInstanceType(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/instance-availability":
		ms.handleGetInstanceAvailabilities(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/instance-availability/"):
		ms.handleCheckInstanceAvailability(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/long-term/periods/instances":
		ms.handleGetInstancePeriods(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/long-term/periods/clusters":
		ms.handleGetClusterPeriods(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/long-term/periods":
		ms.handleGetPeriods(w, r)
	// Container Deployments
	case r.Method == http.MethodGet && r.URL.Path == "/container-deployments":
		ms.handleGetContainerDeployments(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/container-deployments":
		ms.handleCreateContainerDeployment(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/serverless-compute-resources":
		ms.handleGetServerlessComputeResources(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/secrets":
		ms.handleGetSecrets(w, r)
	// Serverless Jobs
	case r.Method == http.MethodGet && r.URL.Path == "/job-deployments":
		ms.handleGetJobDeployments(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/job-deployments":
		ms.handleCreateJobDeployment(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (ms *MockServer) handleAuth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var grantType, clientID, clientSecret string

	// Check content type and parse accordingly
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// Parse JSON body
		var tokenReq struct {
			GrantType    string `json:"grant_type"`
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&tokenReq); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]string{"error": "invalid_json"})
			return
		}
		grantType = tokenReq.GrantType
		clientID = tokenReq.ClientID
		clientSecret = tokenReq.ClientSecret
	} else {
		// Parse form data
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		grantType = r.FormValue("grant_type")
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
	}

	if grantType == "" || clientID == "" || clientSecret == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "invalid_request"})
		return
	}

	// Mock successful authentication
	response := map[string]interface{}{
		"access_token":  "mock_access_token_12345",
		"refresh_token": "mock_refresh_token_67890",
		"token_type":    "Bearer",
		"expires_in":    3600,
		"scope":         "read write",
	}

	writeJSON(w, response)
}

func (ms *MockServer) handleGetInstances(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	instances := []Instance{
		{
			ID:           "inst_123",
			InstanceType: "1V100.6V",
			Image:        "ubuntu-24.04-cuda-12.8-open-docker",
			PricePerHour: 0.50,
			Hostname:     "test-instance",
			Description:  "Test instance",
			Status:       StatusRunning,
			CreatedAt:    time.Now(),
			SSHKeyIDs:    []string{"key_123"},
			CPU:          map[string]interface{}{"description": "6 CPU", "number_of_cores": 6},
			GPU:          map[string]interface{}{"description": "1x Tesla V100 16GB", "number_of_gpus": 1},
			Memory:       map[string]interface{}{"description": "32GB RAM", "size_in_gigabytes": 32},
			Storage:      map[string]interface{}{"description": "100GB SSD"},
			GPUMemory:    map[string]interface{}{"description": "16GB GPU RAM", "size_in_gigabytes": 16},
			Location:     LocationFIN01,
			IsSpot:       false,
			OSName:       "test-instance-os",
			Contract:     "PAY_AS_YOU_GO",
			Pricing:      "FIXED_PRICE",
		},
	}

	writeJSON(w, instances)
}

func (ms *MockServer) handleGetInstance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract instance ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	instanceID := parts[2]

	instance := Instance{
		ID:           instanceID,
		InstanceType: "1V100.6V",
		Image:        "ubuntu-24.04-cuda-12.8-open-docker",
		PricePerHour: 0.50,
		Hostname:     "test-instance",
		Description:  "Test instance",
		Status:       StatusRunning,
		CreatedAt:    time.Now(),
		SSHKeyIDs:    []string{"key_123"},
		CPU:          map[string]interface{}{"description": "6 CPU", "number_of_cores": 6},
		GPU:          map[string]interface{}{"description": "1x Tesla V100 16GB", "number_of_gpus": 1},
		Memory:       map[string]interface{}{"description": "32GB RAM", "size_in_gigabytes": 32},
		Storage:      map[string]interface{}{"description": "100GB SSD"},
		GPUMemory:    map[string]interface{}{"description": "16GB GPU RAM", "size_in_gigabytes": 16},
		Location:     LocationFIN01,
		IsSpot:       false,
		OSName:       instanceID + "-os",
		Contract:     "PAY_AS_YOU_GO",
		Pricing:      "FIXED_PRICE",
	}

	writeJSON(w, instance)
}

func (ms *MockServer) handleCreateInstance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Use LocationCode if provided, otherwise default
	location := req.LocationCode
	if location == "" {
		location = LocationFIN01
	}

	// Use Contract and Pricing if provided, otherwise use defaults
	contract := req.Contract
	if contract == "" {
		contract = "PAY_AS_YOU_GO"
	}

	pricing := req.Pricing
	if pricing == "" {
		pricing = "FIXED_PRICE"
	}

	instance := Instance{
		ID:           "inst_new_123",
		InstanceType: req.InstanceType,
		Image:        req.Image,
		PricePerHour: 0.50,
		Hostname:     req.Hostname,
		Description:  req.Description,
		Status:       StatusPending,
		CreatedAt:    time.Now(),
		SSHKeyIDs:    req.SSHKeyIDs,
		CPU:          map[string]interface{}{"description": "6 CPU", "number_of_cores": 6},
		GPU:          map[string]interface{}{"description": "1x Tesla V100 16GB", "number_of_gpus": 1},
		Memory:       map[string]interface{}{"description": "32GB RAM", "size_in_gigabytes": 32},
		Storage:      map[string]interface{}{"description": "100GB SSD"},
		GPUMemory:    map[string]interface{}{"description": "16GB GPU RAM", "size_in_gigabytes": 16},
		Location:     location,
		IsSpot:       req.IsSpot,
		OSName:       "inst_new_123-os",
		Contract:     contract,
		Pricing:      pricing,
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, instance)
}

func (ms *MockServer) handleInstanceAction(w http.ResponseWriter, r *http.Request) {
	var req InstanceActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Mock successful action - API returns 202 Accepted with empty body
	w.WriteHeader(http.StatusAccepted)
}

func (ms *MockServer) handleGetBalance(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	balance := Balance{
		Amount:   100.50,
		Currency: "USD",
	}

	writeJSON(w, balance)
}

func (ms *MockServer) handleGetSSHKeys(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	keys := []SSHKey{
		{
			ID:          "key_123",
			Name:        "Test Key",
			PublicKey:   "ssh-rsa AAAAB3NzaC1yc2E...",
			Fingerprint: "SHA256:abc123...",
			CreatedAt:   time.Now(),
		},
	}

	writeJSON(w, keys)
}

func (ms *MockServer) handleGetLocations(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	locations := []Location{
		{
			Code:        LocationFIN01,
			Name:        "Finland 01",
			Country:     "Finland",
			CountryCode: "FI",
			Available:   true,
		},
	}

	writeJSON(w, locations)
}

func (ms *MockServer) handleGetSSHKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract SSH key ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	keyID := parts[2]

	ms.mu.RLock()
	key, exists := ms.sshKeys[keyID]
	ms.mu.RUnlock()

	if !exists {
		// Return a default SSH key for unknown IDs
		// For key_new_123, return data that matches the SSH key creation test
		if keyID == "key_new_123" {
			key = SSHKey{
				ID:          keyID,
				Name:        "My New Key",
				PublicKey:   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB...",
				Fingerprint: "SHA256:generated...",
				CreatedAt:   time.Now(),
			}
		} else {
			key = SSHKey{
				ID:          keyID,
				Name:        "Mock SSH Key",
				PublicKey:   "ssh-rsa AAAAB3NzaC1yc2E...",
				Fingerprint: "SHA256:mock...",
				CreatedAt:   time.Now(),
			}
		}
	}

	// Return as an array (to match the real API)
	writeJSON(w, []SSHKey{key})
}

func (ms *MockServer) handleCreateSSHKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	// Parse the request to get SSH key data
	var req struct {
		Name      string `json:"name"`
		PublicKey string `json:"key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Generate ID and store the key
	keyID := "key_new_123"
	key := SSHKey{
		ID:          keyID,
		Name:        req.Name,
		PublicKey:   req.PublicKey,
		Fingerprint: "SHA256:generated...",
		CreatedAt:   time.Now(),
	}

	ms.mu.Lock()
	ms.sshKeys[keyID] = key
	ms.mu.Unlock()

	// Return just the ID as plain text (to match the real API)
	w.WriteHeader(http.StatusCreated)
	writeBytes(w, []byte(keyID))
}

func (ms *MockServer) handleDeleteSSHKey(w http.ResponseWriter, r *http.Request) {
	// Extract SSH key ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	keyID := parts[2]

	ms.mu.Lock()
	delete(ms.sshKeys, keyID)
	ms.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func (ms *MockServer) handleDeleteMultipleSSHKeys(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Keys []string `json:"keys"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ms.mu.Lock()
	for _, keyID := range req.Keys {
		delete(ms.sshKeys, keyID)
	}
	ms.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func (ms *MockServer) handleGetScripts(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	scripts := []StartupScript{
		{
			ID:        "script_123",
			Name:      "Test Script",
			Script:    "#!/bin/bash\necho 'Hello World'",
			CreatedAt: time.Now(),
		},
	}

	writeJSON(w, scripts)
}

func (ms *MockServer) handleGetScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract script ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	scriptID := parts[2]

	ms.mu.RLock()
	script, exists := ms.scripts[scriptID]
	ms.mu.RUnlock()

	if !exists {
		// Return a default script for unknown IDs
		script = StartupScript{
			ID:        scriptID,
			Name:      "Mock Script",
			Script:    "#!/bin/bash\necho 'Hello World'",
			CreatedAt: time.Now(),
		}
	}

	// Return as an array (to match the real API behavior)
	writeJSON(w, []StartupScript{script})
}

func (ms *MockServer) handleCreateScript(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Script string `json:"script"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Generate ID and store the script
	scriptID := "script_new_123"
	script := StartupScript{
		ID:        scriptID,
		Name:      req.Name,
		Script:    req.Script,
		CreatedAt: time.Now(),
	}

	ms.mu.Lock()
	ms.scripts[scriptID] = script
	ms.mu.Unlock()

	// Return plain text ID (matching real API behavior)
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	writeBytes(w, []byte(scriptID))
}

func (ms *MockServer) handleDeleteScript(w http.ResponseWriter, r *http.Request) {
	// Extract script ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	scriptID := parts[2]

	ms.mu.Lock()
	delete(ms.scripts, scriptID)
	ms.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func (ms *MockServer) handleDeleteMultipleScripts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scripts []string `json:"scripts"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ms.mu.Lock()
	for _, scriptID := range req.Scripts {
		delete(ms.scripts, scriptID)
	}
	ms.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (ms *MockServer) handleGetVolumeTypes(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	volumeTypes := []VolumeType{
		{
			Type: "NVMe",
			Price: VolumeTypePrice{
				MonthlyPerGB: 0.12,
				Currency:     "USD",
			},
			IsSharedFS:           false,
			BurstBandwidth:       2000,
			ContinuousBandwidth:  1000,
			InternalNetworkSpeed: 10,
			IOPS:                 "up to 50000",
		},
		{
			Type: "NVMe_Shared",
			Price: VolumeTypePrice{
				MonthlyPerGB: 0.15,
				Currency:     "USD",
			},
			IsSharedFS:           true,
			BurstBandwidth:       3000,
			ContinuousBandwidth:  1500,
			InternalNetworkSpeed: 25,
			IOPS:                 "up to 100000",
		},
		{
			Type: "HDD",
			Price: VolumeTypePrice{
				MonthlyPerGB: 0.05,
				Currency:     "USD",
			},
			IsSharedFS:           false,
			BurstBandwidth:       500,
			ContinuousBandwidth:  250,
			InternalNetworkSpeed: 1,
			IOPS:                 "up to 5000",
		},
	}

	writeJSON(w, volumeTypes)
}

func (ms *MockServer) handleGetClusters(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	clusters := []Cluster{
		{
			ID:           "cluster_123",
			ClusterType:  "8V100.48V",
			Image:        "ubuntu-22.04-cuda-12.0",
			PricePerHour: 2.50,
			Hostname:     "test-cluster",
			Description:  "Test cluster",
			Status:       StatusRunning,
			CreatedAt:    time.Now(),
			SSHKeyIDs:    []string{"key_123"},
			CPU:          map[string]interface{}{"description": "48 CPU", "number_of_cores": 48},
			GPU:          map[string]interface{}{"description": "8x Tesla V100 16GB", "number_of_gpus": 8},
			Memory:       map[string]interface{}{"description": "256GB RAM", "size_in_gigabytes": 256},
			Storage:      map[string]interface{}{"description": "1TB SSD"},
			GPUMemory:    map[string]interface{}{"description": "128GB GPU RAM", "size_in_gigabytes": 128},
			Location:     LocationFIN01,
			OSName:       "test-cluster-os",
			Contract:     "PAY_AS_YOU_GO",
			Pricing:      "FIXED_PRICE",
		},
	}

	writeJSON(w, clusters)
}

func (ms *MockServer) handleGetCluster(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	clusterID := parts[2]

	cluster := Cluster{
		ID:           clusterID,
		ClusterType:  "8V100.48V",
		Image:        "ubuntu-22.04-cuda-12.0",
		PricePerHour: 2.50,
		Hostname:     "test-cluster",
		Description:  "Test cluster",
		Status:       StatusRunning,
		CreatedAt:    time.Now(),
		SSHKeyIDs:    []string{"key_123"},
		CPU:          map[string]interface{}{"description": "48 CPU", "number_of_cores": 48},
		GPU:          map[string]interface{}{"description": "8x Tesla V100 16GB", "number_of_gpus": 8},
		Memory:       map[string]interface{}{"description": "256GB RAM", "size_in_gigabytes": 256},
		Storage:      map[string]interface{}{"description": "1TB SSD"},
		GPUMemory:    map[string]interface{}{"description": "128GB GPU RAM", "size_in_gigabytes": 128},
		Location:     LocationFIN01,
		OSName:       clusterID + "-os",
		Contract:     "PAY_AS_YOU_GO",
		Pricing:      "FIXED_PRICE",
	}

	writeJSON(w, cluster)
}

func (ms *MockServer) handleCreateCluster(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Return JSON with cluster ID
	response := map[string]string{
		"id": "cluster_new_123",
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, response)
}

func (ms *MockServer) handleClusterAction(w http.ResponseWriter, r *http.Request) {
	var req ClusterActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Return 202 Accepted with empty body
	w.WriteHeader(http.StatusAccepted)
}

func (ms *MockServer) handleGetClusterTypes(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	clusterTypes := []ClusterType{
		{
			ClusterType:  "8V100.48V",
			Description:  "8x NVIDIA Tesla V100 with 48 vCPUs",
			PricePerHour: 2.50,
			CPU:          map[string]interface{}{"description": "48 CPU", "number_of_cores": 48},
			GPU:          map[string]interface{}{"description": "8x Tesla V100 16GB", "number_of_gpus": 8},
			Memory:       map[string]interface{}{"description": "256GB RAM", "size_in_gigabytes": 256},
			Storage:      map[string]interface{}{"description": "1TB SSD"},
			GPUMemory:    map[string]interface{}{"description": "128GB GPU RAM", "size_in_gigabytes": 128},
			Manufacturer: "NVIDIA",
			Available:    true,
		},
	}

	writeJSON(w, clusterTypes)
}

func (ms *MockServer) handleGetClusterAvailabilities(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	availabilities := []ClusterAvailability{
		{
			ClusterType:  "8V100.48V",
			LocationCode: LocationFIN01,
			Available:    true,
		},
	}

	writeJSON(w, availabilities)
}

func (ms *MockServer) handleCheckClusterTypeAvailability(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract cluster type from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Return true for availability
	writeJSON(w, true)
}

func (ms *MockServer) handleGetClusterImages(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	images := []ClusterImage{
		{
			Name:        "ubuntu-22.04-cuda-12.0",
			Description: "Ubuntu 22.04 with CUDA 12.0",
			Version:     "1.0",
			Available:   true,
		},
		{
			Name:        "ubuntu-20.04-cuda-11.8",
			Description: "Ubuntu 20.04 with CUDA 11.8",
			Version:     "1.0",
			Available:   true,
		},
	}

	writeJSON(w, images)
}

func (ms *MockServer) handleGetContainerTypes(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	containerTypes := []ContainerType{
		{
			ID:                  "container_type_1",
			Model:               "A100",
			Name:                "1x NVIDIA A100 80GB",
			InstanceType:        "1A100.80G",
			CPU:                 map[string]interface{}{"description": "16 vCPU", "number_of_cores": 16},
			GPU:                 map[string]interface{}{"description": "1x NVIDIA A100 80GB", "number_of_gpus": 1},
			GPUMemory:           map[string]interface{}{"description": "80GB GPU RAM", "size_in_gigabytes": 80},
			Memory:              map[string]interface{}{"description": "64GB RAM", "size_in_gigabytes": 64},
			ServerlessPrice:     0.00123,
			ServerlessSpotPrice: 0.00098,
			Currency:            "usd",
			Manufacturer:        "NVIDIA",
		},
		{
			ID:                  "container_type_2",
			Model:               "V100",
			Name:                "1x NVIDIA V100 16GB",
			InstanceType:        "1V100.16G",
			CPU:                 map[string]interface{}{"description": "8 vCPU", "number_of_cores": 8},
			GPU:                 map[string]interface{}{"description": "1x NVIDIA V100 16GB", "number_of_gpus": 1},
			GPUMemory:           map[string]interface{}{"description": "16GB GPU RAM", "size_in_gigabytes": 16},
			Memory:              map[string]interface{}{"description": "32GB RAM", "size_in_gigabytes": 32},
			ServerlessPrice:     0.00089,
			ServerlessSpotPrice: 0.00067,
			Currency:            "usd",
			Manufacturer:        "NVIDIA",
		},
	}

	writeJSON(w, containerTypes)
}

func (ms *MockServer) handleGetImages(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	images := []Image{
		{
			ID:        "image_1",
			ImageType: "ubuntu-22.04-cuda-12.0",
			Name:      "Ubuntu 22.04 with CUDA 12.0",
			IsDefault: true,
			IsCluster: false,
			Details:   []string{"Ubuntu 22.04 LTS", "CUDA 12.0", "Python 3.10"},
			Category:  "gpu",
		},
		{
			ID:        "image_2",
			ImageType: "pytorch-2.0",
			Name:      "PyTorch 2.0",
			IsDefault: false,
			IsCluster: false,
			Details:   []string{"PyTorch 2.0", "CUDA 11.8", "Python 3.10"},
			Category:  "ml",
		},
	}

	writeJSON(w, images)
}

func (ms *MockServer) handleGetInstanceTypes(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	instanceTypes := []InstanceTypeInfo{
		{
			ID:              "instance_type_1",
			InstanceType:    "1H100.80S.22V",
			Model:           "H100",
			Name:            "1x NVIDIA H100 80GB SXM",
			CPU:             map[string]interface{}{"description": "22 vCPU", "number_of_cores": 22},
			GPU:             map[string]interface{}{"description": "1x NVIDIA H100 80GB SXM", "number_of_gpus": 1},
			GPUMemory:       map[string]interface{}{"description": "80GB GPU RAM", "size_in_gigabytes": 80},
			Memory:          map[string]interface{}{"description": "128GB RAM", "size_in_gigabytes": 128},
			PricePerHour:    3.17,
			SpotPrice:       2.54,
			DynamicPrice:    2.85,
			MaxDynamicPrice: 3.50,
			Storage:         map[string]interface{}{"description": "1TB NVMe SSD"},
			Currency:        "usd",
			Manufacturer:    "NVIDIA",
			BestFor:         []string{"Large Language Models", "AI Training", "High-Performance Computing"},
			Description:     "High-performance GPU instance with NVIDIA H100 80GB SXM",
		},
		{
			ID:              "instance_type_2",
			InstanceType:    "1V100.6V",
			Model:           "V100",
			Name:            "1x NVIDIA V100 16GB",
			CPU:             map[string]interface{}{"description": "6 vCPU", "number_of_cores": 6},
			GPU:             map[string]interface{}{"description": "1x NVIDIA V100 16GB", "number_of_gpus": 1},
			GPUMemory:       map[string]interface{}{"description": "16GB GPU RAM", "size_in_gigabytes": 16},
			Memory:          map[string]interface{}{"description": "48GB RAM", "size_in_gigabytes": 48},
			PricePerHour:    0.89,
			SpotPrice:       0.67,
			DynamicPrice:    0.78,
			MaxDynamicPrice: 0.95,
			Storage:         map[string]interface{}{"description": "500GB NVMe SSD"},
			Currency:        "usd",
			Manufacturer:    "NVIDIA",
			BestFor:         []string{"Deep Learning", "Machine Learning", "Data Science"},
			Description:     "GPU instance with NVIDIA V100 16GB",
		},
	}

	writeJSON(w, instanceTypes)
}

func (ms *MockServer) handleGetInstanceType(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract instance type from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	instanceType := parts[2]

	// Return appropriate instance type based on request
	var response InstanceTypeInfo

	if strings.Contains(instanceType, "H100") {
		response = InstanceTypeInfo{
			ID:              "instance_type_1",
			InstanceType:    instanceType,
			Model:           "H100",
			Name:            "1x NVIDIA H100 80GB SXM",
			CPU:             map[string]interface{}{"description": "22 vCPU", "number_of_cores": 22},
			GPU:             map[string]interface{}{"description": "1x NVIDIA H100 80GB SXM", "number_of_gpus": 1},
			GPUMemory:       map[string]interface{}{"description": "80GB GPU RAM", "size_in_gigabytes": 80},
			Memory:          map[string]interface{}{"description": "128GB RAM", "size_in_gigabytes": 128},
			PricePerHour:    3.17,
			SpotPrice:       2.54,
			DynamicPrice:    2.85,
			MaxDynamicPrice: 3.50,
			Storage:         map[string]interface{}{"description": "1TB NVMe SSD"},
			Currency:        "usd",
			Manufacturer:    "NVIDIA",
			BestFor:         []string{"Large Language Models", "AI Training", "High-Performance Computing"},
			Description:     "High-performance GPU instance with NVIDIA H100 80GB SXM",
		}
	} else if strings.Contains(instanceType, "V100") {
		response = InstanceTypeInfo{
			ID:              "instance_type_2",
			InstanceType:    instanceType,
			Model:           "V100",
			Name:            "1x NVIDIA V100 16GB",
			CPU:             map[string]interface{}{"description": "6 vCPU", "number_of_cores": 6},
			GPU:             map[string]interface{}{"description": "1x NVIDIA V100 16GB", "number_of_gpus": 1},
			GPUMemory:       map[string]interface{}{"description": "16GB GPU RAM", "size_in_gigabytes": 16},
			Memory:          map[string]interface{}{"description": "48GB RAM", "size_in_gigabytes": 48},
			PricePerHour:    0.89,
			SpotPrice:       0.67,
			DynamicPrice:    0.78,
			MaxDynamicPrice: 0.95,
			Storage:         map[string]interface{}{"description": "500GB NVMe SSD"},
			Currency:        "usd",
			Manufacturer:    "NVIDIA",
			BestFor:         []string{"Deep Learning", "Machine Learning", "Data Science"},
			Description:     "GPU instance with NVIDIA V100 16GB",
		}
	} else {
		// Default response for unknown types
		response = InstanceTypeInfo{
			ID:              "instance_type_unknown",
			InstanceType:    instanceType,
			Model:           "Generic",
			Name:            "Generic Instance Type",
			CPU:             map[string]interface{}{"description": "4 vCPU", "number_of_cores": 4},
			GPU:             map[string]interface{}{"description": "1x Generic GPU", "number_of_gpus": 1},
			GPUMemory:       map[string]interface{}{"description": "8GB GPU RAM", "size_in_gigabytes": 8},
			Memory:          map[string]interface{}{"description": "32GB RAM", "size_in_gigabytes": 32},
			PricePerHour:    0.50,
			SpotPrice:       0.40,
			DynamicPrice:    0.45,
			MaxDynamicPrice: 0.55,
			Storage:         map[string]interface{}{"description": "250GB NVMe SSD"},
			Currency:        "usd",
			Manufacturer:    "Generic",
			BestFor:         []string{"General Purpose"},
			Description:     "Generic instance type",
		}
	}

	writeJSON(w, response)
}

func (ms *MockServer) handleGetInstanceAvailabilities(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	availabilities := []LocationAvailability{
		{
			LocationCode:   LocationFIN01,
			Availabilities: []string{"1H100.80S.22V", "1V100.6V", "2V100.12V"},
		},
		{
			LocationCode:   "NOR-01",
			Availabilities: []string{"1V100.6V"},
		},
	}

	writeJSON(w, availabilities)
}

func (ms *MockServer) handleCheckInstanceAvailability(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract instance type from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	instanceType := parts[2]

	// Mock: H100 and V100 types are available
	available := strings.Contains(instanceType, "H100") || strings.Contains(instanceType, "V100")

	writeJSON(w, available)
}

func (ms *MockServer) handleGetPriceHistory(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Mock price history data for the last 30 days
	now := time.Now()
	h100Records := make([]PriceHistoryRecord, 30)
	v100Records := make([]PriceHistoryRecord, 30)

	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i).Format(time.RFC3339)

		h100Records[i] = PriceHistoryRecord{
			Date:                date,
			FixedPricePerHour:   3.17,
			DynamicPricePerHour: 3.17 + float64(i%10)*0.05, // Varies between 3.17-3.62
			Currency:            "usd",
		}

		v100Records[i] = PriceHistoryRecord{
			Date:                date,
			FixedPricePerHour:   0.89,
			DynamicPricePerHour: 0.89 + float64(i%8)*0.03, // Varies between 0.89-1.10
			Currency:            "usd",
		}
	}

	priceHistory := map[string][]PriceHistoryRecord{
		"1H100.80S.22V": h100Records,
		"1V100.6V":      v100Records,
	}

	writeJSON(w, priceHistory)
}

func (ms *MockServer) handleGetInstancePeriods(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	periods := []LongTermPeriod{
		{
			Code:               "3_MONTHS",
			Name:               "3 Months",
			IsEnabled:          true,
			UnitName:           "month",
			UnitValue:          3,
			DiscountPercentage: 5.0,
		},
		{
			Code:               "6_MONTHS",
			Name:               "6 Months",
			IsEnabled:          true,
			UnitName:           "month",
			UnitValue:          6,
			DiscountPercentage: 10.0,
		},
		{
			Code:               "12_MONTHS",
			Name:               "12 Months",
			IsEnabled:          true,
			UnitName:           "month",
			UnitValue:          12,
			DiscountPercentage: 15.0,
		},
	}

	writeJSON(w, periods)
}

func (ms *MockServer) handleGetPeriods(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	periods := []LongTermPeriod{
		{
			Code:               "1_MONTH",
			Name:               "1 Month",
			IsEnabled:          true,
			UnitName:           "month",
			UnitValue:          1,
			DiscountPercentage: 0.0,
		},
		{
			Code:               "3_MONTHS",
			Name:               "3 Months",
			IsEnabled:          true,
			UnitName:           "month",
			UnitValue:          3,
			DiscountPercentage: 5.0,
		},
		{
			Code:               "6_MONTHS",
			Name:               "6 Months",
			IsEnabled:          true,
			UnitName:           "month",
			UnitValue:          6,
			DiscountPercentage: 10.0,
		},
		{
			Code:               "12_MONTHS",
			Name:               "12 Months",
			IsEnabled:          true,
			UnitName:           "month",
			UnitValue:          12,
			DiscountPercentage: 15.0,
		},
	}

	writeJSON(w, periods)
}

func (ms *MockServer) handleGetClusterPeriods(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	periods := []LongTermPeriod{
		{
			Code:               "6_MONTHS",
			Name:               "6 Months",
			IsEnabled:          true,
			UnitName:           "month",
			UnitValue:          6,
			DiscountPercentage: 12.0,
		},
		{
			Code:               "12_MONTHS",
			Name:               "12 Months",
			IsEnabled:          true,
			UnitName:           "month",
			UnitValue:          12,
			DiscountPercentage: 20.0,
		},
	}

	writeJSON(w, periods)
}

// Container Deployments Handlers

func (ms *MockServer) handleGetContainerDeployments(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type ContainerDeployment struct {
		Name      string `json:"name"`
		Image     string `json:"image"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		Replicas  int    `json:"replicas"`
	}

	deployments := []ContainerDeployment{
		{
			Name:      "test-deployment",
			Image:     "nginx:latest",
			Status:    "running",
			CreatedAt: "2024-01-01T00:00:00Z",
			UpdatedAt: "2024-01-01T00:00:00Z",
			Replicas:  2,
		},
	}

	writeJSON(w, deployments)
}

func (ms *MockServer) handleCreateContainerDeployment(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	type ContainerDeployment struct {
		Name      string `json:"name"`
		Image     string `json:"image"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		Replicas  int    `json:"replicas"`
	}

	deployment := ContainerDeployment{
		Name:      "new-deployment",
		Image:     "nginx:latest",
		Status:    "pending",
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2024-01-01T00:00:00Z",
		Replicas:  1,
	}

	writeJSON(w, deployment)
}

func (ms *MockServer) handleGetServerlessComputeResources(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type ComputeResource struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		Available bool   `json:"available"`
		CPU       string `json:"cpu"`
		Memory    string `json:"memory"`
	}

	resources := []ComputeResource{
		{
			Name:      "small",
			Type:      "cpu",
			Available: true,
			CPU:       "2",
			Memory:    "4Gi",
		},
		{
			Name:      "medium",
			Type:      "cpu",
			Available: true,
			CPU:       "4",
			Memory:    "8Gi",
		},
	}

	writeJSON(w, resources)
}

func (ms *MockServer) handleGetSecrets(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type Secret struct {
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	secrets := []Secret{
		{
			Name:      "test-secret",
			CreatedAt: "2024-01-01T00:00:00Z",
			UpdatedAt: "2024-01-01T00:00:00Z",
		},
	}

	writeJSON(w, secrets)
}

// Serverless Jobs Handlers

func (ms *MockServer) handleGetJobDeployments(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type JobDeploymentShortInfo struct {
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
		Compute   string `json:"compute"`
	}

	jobs := []JobDeploymentShortInfo{
		{
			Name:      "test-job",
			CreatedAt: "2024-01-01T00:00:00Z",
			Compute:   "small",
		},
	}

	writeJSON(w, jobs)
}

func (ms *MockServer) handleCreateJobDeployment(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	type JobDeployment struct {
		Name      string `json:"name"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	job := JobDeployment{
		Name:      "new-job",
		Status:    "pending",
		CreatedAt: "2024-01-01T00:00:00Z",
		UpdatedAt: "2024-01-01T00:00:00Z",
	}

	writeJSON(w, job)
}

// Note: CreateTestClient is implemented in test files to avoid circular imports

// writeJSON writes a JSON response, handling errors appropriately for test mocks
func writeJSON(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("mock server: failed to encode JSON response: %v", err)
	}
}

// writeBytes writes raw bytes, handling errors appropriately for test mocks
func writeBytes(w http.ResponseWriter, data []byte) {
	if _, err := w.Write(data); err != nil {
		log.Printf("mock server: failed to write response: %v", err)
	}
}

// ErrorResponse creates a mock error response
func ErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	writeJSON(w, map[string]interface{}{
		"message": message,
		"status":  statusCode,
	})
}
