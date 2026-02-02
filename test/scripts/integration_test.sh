#!/bin/bash

# =============================================================================
# Verda Cloud SDK - Integration Test Assist Script
# =============================================================================
# This script helps test the Verda Cloud API by:
# 1. Checking resource availability
# 2. Selecting the cheapest available option
# 3. Creating resources
# 4. Waiting for completion
# 5. Cleaning up
#
# USAGE:
#   export VERDA_CLIENT_ID="your_client_id"
#   export VERDA_CLIENT_SECRET="your_client_secret"
#   export VERDA_BASE_URL="https://api.verda.com/v1"  # Optional
#   ./integration_test.sh [instance|cluster|container|job|all]
#
# Run from repo root: test/scripts/integration_test.sh [instance|cluster|container|job|all]
#
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration from environment variables
BASE_URL="${VERDA_BASE_URL:-https://api.verda.com/v1}"
CLIENT_ID="${VERDA_CLIENT_ID}"
CLIENT_SECRET="${VERDA_CLIENT_SECRET}"

# Validate required environment variables
if [ -z "$CLIENT_ID" ] || [ -z "$CLIENT_SECRET" ]; then
    echo -e "${RED}Error: VERDA_CLIENT_ID and VERDA_CLIENT_SECRET must be set${NC}"
    echo ""
    echo "Usage:"
    echo "  export VERDA_CLIENT_ID='your_client_id'"
    echo "  export VERDA_CLIENT_SECRET='your_client_secret'"
    echo "  export VERDA_BASE_URL='https://api.verda.com/v1'  # Optional"
    echo "  ./integration_test.sh [instance|cluster|container|job|all]"
    exit 1
fi

# =============================================================================
# UTILITY FUNCTIONS
# =============================================================================

log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_step() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# Get access token
get_access_token() {
    log_info "Getting access token..."

    TOKEN_RESPONSE=$(curl -s -X POST "$BASE_URL/oauth2/token" \
        -H "Content-Type: application/json" \
        -d "{
            \"client_id\": \"$CLIENT_ID\",
            \"client_secret\": \"$CLIENT_SECRET\",
            \"grant_type\": \"client_credentials\"
        }")

    ACCESS_TOKEN=$(echo "$TOKEN_RESPONSE" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)

    if [ -z "$ACCESS_TOKEN" ]; then
        log_error "Failed to get access token"
        echo "Response: $TOKEN_RESPONSE"
        exit 1
    fi

    log_success "Got access token: ${ACCESS_TOKEN:0:20}..."
}

# API request helper
api_get() {
    curl -s -X GET "$BASE_URL$1" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json"
}

api_post() {
    curl -s -X POST "$BASE_URL$1" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$2"
}

api_delete() {
    curl -s -X DELETE "$BASE_URL$1" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
}

# =============================================================================
# INSTANCE TESTS
# =============================================================================

find_cheapest_available_instance() {
    log_step "🔍 Finding Cheapest Available Instance Type"

    log_info "Getting instance types..."
    INSTANCE_TYPES=$(api_get "/instance-types?currency=usd")

    # Parse and sort by spot price
    SORTED_TYPES=$(echo "$INSTANCE_TYPES" | jq -r '.[] | "\(.spot_price_per_hour)|\(.instance_type)"' | sort -t'|' -k1 -n)

    CHEAPEST_INSTANCE=""
    CHEAPEST_PRICE=""

    while IFS='|' read -r price instance_type; do
        [ -z "$instance_type" ] && continue

        log_info "Checking availability for $instance_type (\$${price}/hr)..."

        AVAIL=$(api_get "/instance-availability?instance_type=$instance_type")
        IS_AVAILABLE=$(echo "$AVAIL" | jq -r '.[0].is_available // false')

        if [ "$IS_AVAILABLE" = "true" ]; then
            CHEAPEST_INSTANCE="$instance_type"
            CHEAPEST_PRICE="$price"
            log_success "Found available: $instance_type (\$${price}/hr spot)"
            break
        fi
    done <<< "$SORTED_TYPES"

    if [ -z "$CHEAPEST_INSTANCE" ]; then
        log_warning "No instance types currently available"
        return 1
    fi

    echo "$CHEAPEST_INSTANCE|$CHEAPEST_PRICE"
}

wait_for_instance() {
    local INSTANCE_ID="$1"
    local TARGET_STATUS="$2"
    local TIMEOUT="${3:-600}"
    local POLL_INTERVAL=10
    local ELAPSED=0

    log_info "Waiting for instance $INSTANCE_ID to reach status '$TARGET_STATUS' (timeout: ${TIMEOUT}s)..."

    while [ $ELAPSED -lt $TIMEOUT ]; do
        INSTANCE=$(api_get "/instances/$INSTANCE_ID")
        CURRENT_STATUS=$(echo "$INSTANCE" | jq -r '.status')

        log_info "  Current status: $CURRENT_STATUS (${ELAPSED}s elapsed)"

        if [ "$CURRENT_STATUS" = "$TARGET_STATUS" ]; then
            log_success "Instance reached target status '$TARGET_STATUS'"
            return 0
        fi

        if [ "$CURRENT_STATUS" = "FAILED" ] || [ "$CURRENT_STATUS" = "failed" ]; then
            log_error "Instance failed!"
            return 1
        fi

        sleep $POLL_INTERVAL
        ELAPSED=$((ELAPSED + POLL_INTERVAL))
    done

    log_error "Timeout waiting for instance to reach '$TARGET_STATUS'"
    return 1
}

test_instance_crud() {
    log_step "🖥️  Testing Instance CRUD Operations"

    # Find cheapest available
    RESULT=$(find_cheapest_available_instance)
    if [ $? -ne 0 ]; then
        log_warning "Skipping instance test - no availability"
        return 0
    fi

    INSTANCE_TYPE=$(echo "$RESULT" | cut -d'|' -f1)
    PRICE=$(echo "$RESULT" | cut -d'|' -f2)

    log_info "Using instance type: $INSTANCE_TYPE (\$${PRICE}/hr)"

    # Get SSH keys
    SSH_KEYS=$(api_get "/ssh-keys")
    SSH_KEY_ID=$(echo "$SSH_KEYS" | jq -r '.[0].id // empty')

    if [ -z "$SSH_KEY_ID" ]; then
        log_warning "No SSH keys found. Creating test requires SSH key."
        return 0
    fi

    # Create instance
    INSTANCE_NAME="sdk-test-$(date +%s)"
    log_info "Creating instance: $INSTANCE_NAME"

    CREATE_RESPONSE=$(api_post "/instances" "{
        \"instance_type\": \"$INSTANCE_TYPE\",
        \"hostname\": \"$INSTANCE_NAME\",
        \"ssh_key_ids\": [\"$SSH_KEY_ID\"],
        \"description\": \"SDK integration test\",
        \"location_code\": \"FIN-01\"
    }")

    INSTANCE_ID=$(echo "$CREATE_RESPONSE" | jq -r '.id // empty')

    if [ -z "$INSTANCE_ID" ]; then
        log_error "Failed to create instance"
        echo "$CREATE_RESPONSE"
        return 1
    fi

    log_success "Created instance: $INSTANCE_ID"

    # Cleanup function
    cleanup_instance() {
        log_info "Cleaning up instance $INSTANCE_ID..."
        api_delete "/instances/$INSTANCE_ID"
        log_success "Instance deleted"
    }
    trap cleanup_instance EXIT

    # Wait for running status
    if wait_for_instance "$INSTANCE_ID" "running" 600; then
        log_success "Instance is running!"

        # List instances
        log_info "Listing instances..."
        INSTANCES=$(api_get "/instances")
        INSTANCE_COUNT=$(echo "$INSTANCES" | jq 'length')
        log_success "Found $INSTANCE_COUNT instances"

        # Get instance details
        log_info "Getting instance details..."
        DETAILS=$(api_get "/instances/$INSTANCE_ID")
        log_success "Instance details retrieved"
    fi

    # Cleanup happens via trap
    log_success "Instance CRUD test complete!"
}

# =============================================================================
# CLUSTER TESTS
# =============================================================================

find_cheapest_available_cluster() {
    log_step "🔍 Finding Cheapest Available Cluster Type"

    log_info "Getting cluster types..."
    CLUSTER_TYPES=$(api_get "/cluster-types?currency=usd")

    log_info "Getting cluster availability..."
    AVAILABILITIES=$(api_get "/cluster-availability")

    # Find cheapest available
    CHEAPEST_CLUSTER=""
    CHEAPEST_PRICE=999999
    CHEAPEST_LOCATION=""

    for row in $(echo "$CLUSTER_TYPES" | jq -r '.[] | @base64'); do
        _jq() {
            echo "${row}" | base64 --decode | jq -r "${1}"
        }

        CLUSTER_TYPE=$(_jq '.cluster_type')
        PRICE=$(_jq '.price_per_hour')

        # Check if this cluster type is available
        IS_AVAILABLE=$(echo "$AVAILABILITIES" | jq -r --arg ct "$CLUSTER_TYPE" '.[] | select(.cluster_type == $ct and .available == true) | .location_code' | head -1)

        if [ -n "$IS_AVAILABLE" ]; then
            PRICE_INT=$(echo "$PRICE * 100" | bc | cut -d'.' -f1)
            CHEAPEST_INT=$(echo "$CHEAPEST_PRICE * 100" | bc | cut -d'.' -f1)

            if [ "$PRICE_INT" -lt "$CHEAPEST_INT" ]; then
                CHEAPEST_CLUSTER="$CLUSTER_TYPE"
                CHEAPEST_PRICE="$PRICE"
                CHEAPEST_LOCATION="$IS_AVAILABLE"
                log_info "  Found: $CLUSTER_TYPE at $IS_AVAILABLE (\$${PRICE}/hr)"
            fi
        fi
    done

    if [ -z "$CHEAPEST_CLUSTER" ]; then
        log_warning "No cluster types currently available"
        return 1
    fi

    log_success "Cheapest available: $CHEAPEST_CLUSTER at $CHEAPEST_LOCATION (\$${CHEAPEST_PRICE}/hr)"
    echo "$CHEAPEST_CLUSTER|$CHEAPEST_PRICE|$CHEAPEST_LOCATION"
}

wait_for_cluster() {
    local CLUSTER_ID="$1"
    local TARGET_STATUS="$2"
    local TIMEOUT="${3:-900}"
    local POLL_INTERVAL=15
    local ELAPSED=0

    log_info "Waiting for cluster $CLUSTER_ID to reach status '$TARGET_STATUS' (timeout: ${TIMEOUT}s)..."

    while [ $ELAPSED -lt $TIMEOUT ]; do
        CLUSTER=$(api_get "/clusters/$CLUSTER_ID")
        CURRENT_STATUS=$(echo "$CLUSTER" | jq -r '.status')

        log_info "  Current status: $CURRENT_STATUS (${ELAPSED}s elapsed)"

        if [ "$CURRENT_STATUS" = "$TARGET_STATUS" ]; then
            log_success "Cluster reached target status '$TARGET_STATUS'"
            return 0
        fi

        if [ "$CURRENT_STATUS" = "FAILED" ] || [ "$CURRENT_STATUS" = "failed" ]; then
            log_error "Cluster failed!"
            return 1
        fi

        sleep $POLL_INTERVAL
        ELAPSED=$((ELAPSED + POLL_INTERVAL))
    done

    log_error "Timeout waiting for cluster to reach '$TARGET_STATUS'"
    return 1
}

test_cluster_crud() {
    log_step "🗄️  Testing Cluster CRUD Operations"

    # Find cheapest available
    RESULT=$(find_cheapest_available_cluster)
    if [ $? -ne 0 ]; then
        log_warning "Skipping cluster test - no availability"
        return 0
    fi

    CLUSTER_TYPE=$(echo "$RESULT" | cut -d'|' -f1)
    PRICE=$(echo "$RESULT" | cut -d'|' -f2)
    LOCATION=$(echo "$RESULT" | cut -d'|' -f3)

    log_info "Using cluster type: $CLUSTER_TYPE at $LOCATION (\$${PRICE}/hr)"

    # Get cluster images
    log_info "Getting cluster images..."
    IMAGES=$(api_get "/images/cluster")
    IMAGE_NAME=$(echo "$IMAGES" | jq -r '.[0].name // empty')

    if [ -z "$IMAGE_NAME" ]; then
        log_warning "No cluster images found"
        return 0
    fi

    # Get SSH keys
    SSH_KEYS=$(api_get "/ssh-keys")
    SSH_KEY_ID=$(echo "$SSH_KEYS" | jq -r '.[0].id // empty')

    if [ -z "$SSH_KEY_ID" ]; then
        log_warning "No SSH keys found. Creating cluster requires SSH key."
        return 0
    fi

    # Create cluster
    CLUSTER_NAME="sdk-test-$(date +%s)"
    log_info "Creating cluster: $CLUSTER_NAME"

    CREATE_RESPONSE=$(api_post "/clusters" "{
        \"cluster_type\": \"$CLUSTER_TYPE\",
        \"hostname\": \"$CLUSTER_NAME\",
        \"image\": \"$IMAGE_NAME\",
        \"ssh_key_ids\": [\"$SSH_KEY_ID\"],
        \"description\": \"SDK integration test\",
        \"location_code\": \"$LOCATION\"
    }")

    CLUSTER_ID=$(echo "$CREATE_RESPONSE" | jq -r '.id // empty')

    if [ -z "$CLUSTER_ID" ]; then
        log_error "Failed to create cluster"
        echo "$CREATE_RESPONSE"
        return 1
    fi

    log_success "Created cluster: $CLUSTER_ID"

    # Cleanup function
    cleanup_cluster() {
        log_info "Cleaning up cluster $CLUSTER_ID..."
        # First discontinue, then delete
        api_post "/clusters/$CLUSTER_ID/action" '{"action": "discontinue"}'
        sleep 5
        api_delete "/clusters/$CLUSTER_ID"
        log_success "Cluster deleted"
    }
    trap cleanup_cluster EXIT

    # Wait for running status
    if wait_for_cluster "$CLUSTER_ID" "running" 900; then
        log_success "Cluster is running!"

        # List clusters
        log_info "Listing clusters..."
        CLUSTERS=$(api_get "/clusters")
        CLUSTER_COUNT=$(echo "$CLUSTERS" | jq 'length')
        log_success "Found $CLUSTER_COUNT clusters"
    fi

    # Cleanup happens via trap
    log_success "Cluster CRUD test complete!"
}

# =============================================================================
# CONTAINER DEPLOYMENT TESTS
# =============================================================================

find_available_container_compute() {
    log_step "🔍 Finding Available Container Compute"

    log_info "Getting serverless compute resources..."
    RESOURCES=$(api_get "/serverless-compute-resources")

    # Find first available
    for row in $(echo "$RESOURCES" | jq -r '.[] | @base64'); do
        _jq() {
            echo "${row}" | base64 --decode | jq -r "${1}"
        }

        NAME=$(_jq '.name')
        IS_AVAILABLE=$(_jq '.is_available')
        SIZE=$(_jq '.size')

        if [ "$IS_AVAILABLE" = "true" ]; then
            log_success "Found available: $NAME (size: $SIZE)"
            echo "$NAME"
            return 0
        fi
    done

    log_warning "No container compute resources available"
    return 1
}

wait_for_container() {
    local DEPLOYMENT_NAME="$1"
    local TARGET_STATUS="$2"
    local TIMEOUT="${3:-300}"
    local POLL_INTERVAL=10
    local ELAPSED=0

    log_info "Waiting for container $DEPLOYMENT_NAME to reach status '$TARGET_STATUS' (timeout: ${TIMEOUT}s)..."

    while [ $ELAPSED -lt $TIMEOUT ]; do
        DEPLOYMENT=$(api_get "/container-deployments/$DEPLOYMENT_NAME")
        CURRENT_STATUS=$(echo "$DEPLOYMENT" | jq -r '.status // "unknown"')

        log_info "  Current status: $CURRENT_STATUS (${ELAPSED}s elapsed)"

        if [ "$CURRENT_STATUS" = "$TARGET_STATUS" ]; then
            log_success "Container reached target status '$TARGET_STATUS'"
            return 0
        fi

        if [ "$CURRENT_STATUS" = "FAILED" ] || [ "$CURRENT_STATUS" = "failed" ]; then
            log_error "Container deployment failed!"
            return 1
        fi

        sleep $POLL_INTERVAL
        ELAPSED=$((ELAPSED + POLL_INTERVAL))
    done

    log_error "Timeout waiting for container to reach '$TARGET_STATUS'"
    return 1
}

test_container_crud() {
    log_step "📦 Testing Container Deployment CRUD Operations"

    # Find available compute
    COMPUTE_NAME=$(find_available_container_compute)
    if [ $? -ne 0 ]; then
        log_warning "Skipping container test - no availability"
        return 0
    fi

    log_info "Using compute: $COMPUTE_NAME"

    # Create container deployment
    DEPLOYMENT_NAME="sdk-test-$(date +%s)"
    log_info "Creating container deployment: $DEPLOYMENT_NAME"

    CREATE_RESPONSE=$(api_post "/container-deployments" "{
        \"name\": \"$DEPLOYMENT_NAME\",
        \"isSpot\": false,
        \"compute\": {
            \"name\": \"$COMPUTE_NAME\",
            \"size\": 1
        },
        \"containerRegistrySettings\": {
            \"isPrivate\": false
        },
        \"scaling\": {
            \"minReplicaCount\": 1,
            \"maxReplicaCount\": 1,
            \"scaleDownPolicy\": {
                \"delaySeconds\": 300
            },
            \"scaleUpPolicy\": {
                \"delaySeconds\": 60
            },
            \"queueMessageTTLSeconds\": 300,
            \"concurrentRequestsPerReplica\": 1,
            \"scalingTriggers\": {
                \"queueLoad\": {
                    \"threshold\": 1
                },
                \"cpuUtilization\": {
                    \"enabled\": true,
                    \"threshold\": 80
                },
                \"gpuUtilization\": {
                    \"enabled\": true,
                    \"threshold\": 80
                }
            }
        },
        \"containers\": [
            {
                \"image\": \"registry-1.docker.io/library/nginx:1.25.3\",
                \"exposedPort\": 80
            }
        ]
    }")

    # Check for error
    ERROR=$(echo "$CREATE_RESPONSE" | jq -r '.message // empty')
    if [ -n "$ERROR" ]; then
        log_error "Failed to create container deployment: $ERROR"
        echo "$CREATE_RESPONSE"
        return 1
    fi

    log_success "Created container deployment: $DEPLOYMENT_NAME"

    # Cleanup function
    cleanup_container() {
        log_info "Cleaning up container deployment $DEPLOYMENT_NAME..."
        api_delete "/container-deployments/$DEPLOYMENT_NAME?timeout=60000"
        log_success "Container deployment deleted"
    }
    trap cleanup_container EXIT

    # Wait for deployment to be ready
    sleep 10

    # List deployments
    log_info "Listing container deployments..."
    DEPLOYMENTS=$(api_get "/container-deployments")
    DEPLOYMENT_COUNT=$(echo "$DEPLOYMENTS" | jq 'length')
    log_success "Found $DEPLOYMENT_COUNT container deployments"

    # Get deployment details
    log_info "Getting deployment details..."
    DETAILS=$(api_get "/container-deployments/$DEPLOYMENT_NAME")
    log_success "Deployment details retrieved"

    # Cleanup happens via trap
    log_success "Container CRUD test complete!"
}

# =============================================================================
# SERVERLESS JOB TESTS
# =============================================================================

test_job_crud() {
    log_step "⚡ Testing Serverless Job CRUD Operations"

    # Find available compute
    COMPUTE_NAME=$(find_available_container_compute)
    if [ $? -ne 0 ]; then
        log_warning "Skipping job test - no availability"
        return 0
    fi

    log_info "Using compute: $COMPUTE_NAME"

    # Create job deployment
    JOB_NAME="sdk-job-test-$(date +%s)"
    log_info "Creating job deployment: $JOB_NAME"

    CREATE_RESPONSE=$(api_post "/job-deployments" "{
        \"name\": \"$JOB_NAME\",
        \"containerRegistrySettings\": {
            \"isPrivate\": false
        },
        \"containers\": [
            {
                \"image\": \"registry-1.docker.io/library/alpine:3.19\",
                \"exposedPort\": 8080,
                \"entrypointOverrides\": {
                    \"enabled\": true,
                    \"cmd\": [\"echo\", \"hello from SDK test\"]
                }
            }
        ],
        \"compute\": {
            \"name\": \"$COMPUTE_NAME\",
            \"size\": 1
        },
        \"scaling\": {
            \"maxReplicaCount\": 1,
            \"queueMessageTTLSeconds\": 300,
            \"deadlineSeconds\": 3600
        }
    }")

    # Check for error
    ERROR=$(echo "$CREATE_RESPONSE" | jq -r '.message // empty')
    if [ -n "$ERROR" ]; then
        log_error "Failed to create job deployment: $ERROR"
        echo "$CREATE_RESPONSE"
        return 1
    fi

    log_success "Created job deployment: $JOB_NAME"

    # Cleanup function
    cleanup_job() {
        log_info "Cleaning up job deployment $JOB_NAME..."
        api_delete "/job-deployments/$JOB_NAME?timeout=60000"
        log_success "Job deployment deleted"
    }
    trap cleanup_job EXIT

    # Wait for deployment
    sleep 10

    # List job deployments
    log_info "Listing job deployments..."
    JOBS=$(api_get "/job-deployments")
    JOB_COUNT=$(echo "$JOBS" | jq 'length')
    log_success "Found $JOB_COUNT job deployments"

    # Get job details
    log_info "Getting job details..."
    DETAILS=$(api_get "/job-deployments/$JOB_NAME")
    log_success "Job details retrieved"

    # Cleanup happens via trap
    log_success "Job CRUD test complete!"
}

# =============================================================================
# MAIN
# =============================================================================

main() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║         Verda Cloud SDK - Integration Test Assist                  ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo ""
    echo "Base URL: $BASE_URL"
    echo "Test Type: ${1:-all}"
    echo ""

    # Get access token
    get_access_token

    TEST_TYPE="${1:-all}"

    case "$TEST_TYPE" in
        instance)
            test_instance_crud
            ;;
        cluster)
            test_cluster_crud
            ;;
        container)
            test_container_crud
            ;;
        job)
            test_job_crud
            ;;
        all)
            test_instance_crud
            echo ""
            echo "⏳ Waiting 2 minutes before next test..."
            sleep 120

            test_cluster_crud
            echo ""
            echo "⏳ Waiting 2 minutes before next test..."
            sleep 120

            test_container_crud
            echo ""
            echo "⏳ Waiting 2 minutes before next test..."
            sleep 120

            test_job_crud
            ;;
        *)
            echo "Unknown test type: $TEST_TYPE"
            echo "Usage: $0 [instance|cluster|container|job|all]"
            exit 1
            ;;
    esac

    echo ""
    log_success "All tests completed!"
}

main "$@"
