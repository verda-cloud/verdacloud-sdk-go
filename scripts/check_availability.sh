#!/bin/bash

# =============================================================================
# Verda Cloud SDK - Availability Check Script
# =============================================================================
# This script checks resource availability and displays pricing information
# for instances, clusters, and serverless compute.
#
# USAGE:
#   export VERDA_CLIENT_ID="your_client_id"
#   export VERDA_CLIENT_SECRET="your_client_secret"
#   export VERDA_BASE_URL="https://api.verda.com/v1"  # Optional
#   ./check_availability.sh [instance|cluster|container|all]
#
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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
    echo "  ./check_availability.sh [instance|cluster|container|all]"
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
    
    log_success "Authenticated successfully"
    echo ""
}

# API request helper
api_get() {
    curl -s -X GET "$BASE_URL$1" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json"
}

# =============================================================================
# INSTANCE AVAILABILITY
# =============================================================================

check_instance_availability() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║                    Instance Type Availability                      ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo ""
    
    log_info "Fetching instance types..."
    INSTANCE_TYPES=$(api_get "/instance-types?currency=usd")
    
    # Sort by spot price
    SORTED=$(echo "$INSTANCE_TYPES" | jq -r 'sort_by(.spot_price_per_hour) | .[] | "\(.instance_type)|\(.spot_price_per_hour)|\(.price_per_hour)"')
    
    AVAILABLE_COUNT=0
    TOTAL_COUNT=0
    
    printf "${CYAN}%-25s %-12s %-12s %-12s${NC}\n" "Instance Type" "Spot Price" "On-Demand" "Available"
    printf "%-25s %-12s %-12s %-12s\n" "-------------------------" "------------" "------------" "------------"
    
    while IFS='|' read -r instance_type spot_price regular_price; do
        [ -z "$instance_type" ] && continue
        TOTAL_COUNT=$((TOTAL_COUNT + 1))
        
        AVAIL=$(api_get "/instance-availability?instance_type=$instance_type")
        IS_AVAILABLE=$(echo "$AVAIL" | jq -r '.[0].is_available // false')
        
        if [ "$IS_AVAILABLE" = "true" ]; then
            AVAILABLE_COUNT=$((AVAILABLE_COUNT + 1))
            printf "%-25s \$%-11s \$%-11s ${GREEN}%-12s${NC}\n" "$instance_type" "$spot_price/hr" "$regular_price/hr" "YES"
        else
            printf "%-25s \$%-11s \$%-11s ${RED}%-12s${NC}\n" "$instance_type" "$spot_price/hr" "$regular_price/hr" "NO"
        fi
    done <<< "$SORTED"
    
    echo ""
    echo "Summary: $AVAILABLE_COUNT of $TOTAL_COUNT instance types available"
    
    # Find cheapest available
    echo ""
    log_info "Finding cheapest available instance type..."
    
    while IFS='|' read -r instance_type spot_price regular_price; do
        [ -z "$instance_type" ] && continue
        
        AVAIL=$(api_get "/instance-availability?instance_type=$instance_type")
        IS_AVAILABLE=$(echo "$AVAIL" | jq -r '.[0].is_available // false')
        
        if [ "$IS_AVAILABLE" = "true" ]; then
            log_success "Cheapest available: $instance_type (\$${spot_price}/hr spot)"
            break
        fi
    done <<< "$SORTED"
}

# =============================================================================
# CLUSTER AVAILABILITY
# =============================================================================

check_cluster_availability() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║                     Cluster Type Availability                      ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo ""
    
    log_info "Fetching cluster types..."
    CLUSTER_TYPES=$(api_get "/cluster-types?currency=usd")
    
    log_info "Fetching cluster availability..."
    AVAILABILITIES=$(api_get "/cluster-availability")
    
    # Sort by price
    SORTED=$(echo "$CLUSTER_TYPES" | jq -r 'sort_by(.price_per_hour) | .[] | "\(.cluster_type)|\(.price_per_hour)|\(.gpu_type)|\(.cpu_count)|\(.memory)"')
    
    AVAILABLE_COUNT=0
    TOTAL_COUNT=0
    
    printf "${CYAN}%-20s %-12s %-15s %-10s %-10s %-12s${NC}\n" "Cluster Type" "Price" "GPU" "CPUs" "Memory" "Available"
    printf "%-20s %-12s %-15s %-10s %-10s %-12s\n" "--------------------" "------------" "---------------" "----------" "----------" "------------"
    
    while IFS='|' read -r cluster_type price gpu_type cpu_count memory; do
        [ -z "$cluster_type" ] && continue
        TOTAL_COUNT=$((TOTAL_COUNT + 1))
        
        # Check if available at any location
        LOCATION=$(echo "$AVAILABILITIES" | jq -r --arg ct "$cluster_type" '.[] | select(.cluster_type == $ct and .available == true) | .location_code' | head -1)
        
        if [ -n "$LOCATION" ]; then
            AVAILABLE_COUNT=$((AVAILABLE_COUNT + 1))
            printf "%-20s \$%-11s %-15s %-10s %-10s ${GREEN}%-12s${NC}\n" "$cluster_type" "$price/hr" "$gpu_type" "$cpu_count" "$memory" "YES ($LOCATION)"
        else
            printf "%-20s \$%-11s %-15s %-10s %-10s ${RED}%-12s${NC}\n" "$cluster_type" "$price/hr" "$gpu_type" "$cpu_count" "$memory" "NO"
        fi
    done <<< "$SORTED"
    
    echo ""
    echo "Summary: $AVAILABLE_COUNT of $TOTAL_COUNT cluster types available"
    
    # Find cheapest available
    echo ""
    log_info "Finding cheapest available cluster type..."
    
    while IFS='|' read -r cluster_type price gpu_type cpu_count memory; do
        [ -z "$cluster_type" ] && continue
        
        LOCATION=$(echo "$AVAILABILITIES" | jq -r --arg ct "$cluster_type" '.[] | select(.cluster_type == $ct and .available == true) | .location_code' | head -1)
        
        if [ -n "$LOCATION" ]; then
            log_success "Cheapest available: $cluster_type at $LOCATION (\$${price}/hr)"
            break
        fi
    done <<< "$SORTED"
}

# =============================================================================
# CONTAINER COMPUTE AVAILABILITY
# =============================================================================

check_container_availability() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║                Container Compute Availability                      ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo ""
    
    log_info "Fetching serverless compute resources..."
    RESOURCES=$(api_get "/serverless-compute-resources")
    
    AVAILABLE_COUNT=0
    TOTAL_COUNT=0
    
    printf "${CYAN}%-25s %-15s %-12s${NC}\n" "Compute Name" "Size" "Available"
    printf "%-25s %-15s %-12s\n" "-------------------------" "---------------" "------------"
    
    for row in $(echo "$RESOURCES" | jq -r '.[] | @base64'); do
        _jq() {
            echo "${row}" | base64 --decode | jq -r "${1}"
        }
        
        NAME=$(_jq '.name')
        IS_AVAILABLE=$(_jq '.is_available')
        SIZE=$(_jq '.size')
        
        TOTAL_COUNT=$((TOTAL_COUNT + 1))
        
        if [ "$IS_AVAILABLE" = "true" ]; then
            AVAILABLE_COUNT=$((AVAILABLE_COUNT + 1))
            printf "%-25s %-15s ${GREEN}%-12s${NC}\n" "$NAME" "$SIZE" "YES"
        else
            printf "%-25s %-15s ${RED}%-12s${NC}\n" "$NAME" "$SIZE" "NO"
        fi
    done
    
    echo ""
    echo "Summary: $AVAILABLE_COUNT of $TOTAL_COUNT compute resources available"
    
    if [ "$AVAILABLE_COUNT" -gt 0 ]; then
        echo ""
        log_success "Container/Job deployments can be created with available compute resources"
    else
        log_warning "No compute resources available for container/job deployments"
    fi
}

# =============================================================================
# MAIN
# =============================================================================

main() {
    echo ""
    echo "╔═══════════════════════════════════════════════════════════════════╗"
    echo "║          Verda Cloud SDK - Availability Check                      ║"
    echo "╚═══════════════════════════════════════════════════════════════════╝"
    echo ""
    echo "Base URL: $BASE_URL"
    echo ""
    
    # Get access token
    get_access_token
    
    CHECK_TYPE="${1:-all}"
    
    case "$CHECK_TYPE" in
        instance)
            check_instance_availability
            ;;
        cluster)
            check_cluster_availability
            ;;
        container)
            check_container_availability
            ;;
        all)
            check_instance_availability
            check_cluster_availability
            check_container_availability
            ;;
        *)
            echo "Unknown check type: $CHECK_TYPE"
            echo "Usage: $0 [instance|cluster|container|all]"
            exit 1
            ;;
    esac
    
    echo ""
    log_success "Availability check complete!"
}

main "$@"
