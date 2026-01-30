# Integration Test Scripts

This folder contains helper scripts for testing the Verda Cloud API.

## Prerequisites

- `curl` installed
- `jq` installed (for JSON parsing)
- `bc` installed (for price calculations)
- Valid Verda Cloud API credentials

## Environment Variables

All scripts require these environment variables:

```bash
export VERDA_CLIENT_ID="your_client_id"
export VERDA_CLIENT_SECRET="your_client_secret"
export VERDA_BASE_URL="https://api.verda.com/v1"  # Optional, defaults to production
```

**IMPORTANT**: Never hardcode credentials in scripts. Always use environment variables.

## Scripts

### check_availability.sh

Checks availability for instances, clusters, and serverless compute resources.

```bash
# Check all resources
./check_availability.sh

# Check specific resource type
./check_availability.sh instance
./check_availability.sh cluster
./check_availability.sh container
```

**Features:**
- Lists all resource types with pricing
- Shows availability status for each
- Identifies the cheapest available option
- Color-coded output (green=available, red=unavailable)

### integration_test.sh

Runs full CRUD integration tests with smart resource selection.

```bash
# Run all tests (with 2-minute wait between each)
./integration_test.sh all

# Run specific test
./integration_test.sh instance
./integration_test.sh cluster
./integration_test.sh container
./integration_test.sh job
```

**Features:**
- Checks availability before creating resources
- Selects the cheapest available resource type
- Creates, reads, lists resources
- Waits for resources to reach ready status
- Automatically cleans up resources on completion/error
- Color-coded output with progress indicators

## Test Flow

1. **Authentication** - Gets OAuth2 access token
2. **Availability Check** - Finds cheapest available resource type
3. **Create** - Creates the resource with proper configuration
4. **Wait** - Polls until resource reaches target status
5. **List/Read** - Verifies resource is accessible
6. **Cleanup** - Deletes resource (runs even on error via trap)

## Troubleshooting

### No resources available
The API may have limited availability. Try:
- Different time of day
- Different resource types
- Check your account quota

### Authentication failed
- Verify CLIENT_ID and CLIENT_SECRET are correct
- Check if credentials have expired
- Ensure BASE_URL is correct

### jq: command not found
Install jq:
```bash
# macOS
brew install jq

# Ubuntu/Debian
apt-get install jq

# CentOS/RHEL
yum install jq
```

## Security Notes

- Scripts read credentials from environment variables only
- No credentials are logged or stored
- Access tokens are truncated in output (first 20 chars only)
- Use production URL only when intentional
