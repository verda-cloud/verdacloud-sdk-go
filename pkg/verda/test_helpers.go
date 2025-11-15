package verda

import "github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"

// NewTestClient creates a test client using the testutil configuration approach
// This is the standard way to create test clients for unit tests
//
// Example usage:
//
//	mockServer := testutil.NewMockServer()
//	defer mockServer.Close()
//	client := NewTestClient(mockServer)
//	// Use client in tests...
func NewTestClient(mockServer *testutil.MockServer) *Client {
	config := testutil.NewTestClientConfig(mockServer)
	client, _ := NewClient(
		WithBaseURL(config.BaseURL),
		WithClientID(config.ClientID),
		WithClientSecret(config.ClientSecret),
	)
	return client
}
