package verda

import (
	"context"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestLocationsService_Get(t *testing.T) {
	mockServer := testutil.NewMockServer()
	defer mockServer.Close()

	client := NewTestClient(mockServer)

	t.Run("get all locations", func(t *testing.T) {
		ctx := context.Background()
		locations, err := client.Locations.Get(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(locations) == 0 {
			t.Error("expected at least one location")
		}

		location := locations[0]
		if location.Code != LocationFIN01 {
			t.Errorf("expected location code '%s', got '%s'", LocationFIN01, location.Code)
		}

		if location.Name == "" {
			t.Error("expected location name to not be empty")
		}

		if location.CountryCode == "" {
			t.Error("expected country code to not be empty")
		}
	})
}
