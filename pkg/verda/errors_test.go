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
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestAPIError_Error(t *testing.T) {
	t.Run("error with message", func(t *testing.T) {
		err := &APIError{
			StatusCode: 404,
			Message:    "Resource not found",
		}

		expected := "API error 404: Resource not found"
		if err.Error() != expected {
			t.Errorf("expected error message '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("error with details", func(t *testing.T) {
		err := &APIError{
			StatusCode: 400,
			Message:    "Bad request",
			Details:    "Invalid parameter",
		}

		expected := "API error 400: Bad request (Invalid parameter)"
		if err.Error() != expected {
			t.Errorf("expected error message '%s', got '%s'", expected, err.Error())
		}
	})
}

func TestValidationError_Error(t *testing.T) {
	t.Run("validation error", func(t *testing.T) {
		err := &ValidationError{
			Field:   "email",
			Message: "Invalid email format",
		}

		expected := "validation error for field 'email': Invalid email format"
		if err.Error() != expected {
			t.Errorf("expected error message '%s', got '%s'", expected, err.Error())
		}
	})
}

func TestParseAPIError(t *testing.T) {
	longHTML := "<html>" + strings.Repeat("x", 2000) + "</html>"

	cases := []struct {
		name    string
		status  int
		body    string
		code    string
		message string
		details string
	}{
		{"code and message", 401, `{"code":"unauthorized_request","message":"Access token is missing or invalid"}`,
			"unauthorized_request", "Access token is missing or invalid", ""},
		{"code, message and details", 400, `{"code":"invalid_request","message":"name is required","details":"field: name"}`,
			"invalid_request", "name is required", "field: name"},
		{"object details", 400, `{"code":"invalid_request","message":"bad input","details":{"field": "name"}}`,
			"invalid_request", "bad input", `{"field":"name"}`},
		{"message only", 404, `{"message":"cluster not found"}`, "", "cluster not found", ""},
		{"error only", 403, `{"error":"forbidden"}`, "", "forbidden", ""},
		{"message wins over error", 404, `{"statusCode":404,"message":"Cannot GET /x","error":"Not Found"}`,
			"", "Cannot GET /x", ""},
		{"list of messages", 400, `{"message":["name must be a string","size must be positive"],"error":"Bad Request"}`,
			"", "name must be a string; size must be positive", ""},
		{"numeric code", 400, `{"code":42,"message":"bad"}`, "42", "bad", ""},
		{"code without message", 409, `{"code":"conflict"}`, "conflict", `{"code":"conflict"}`, ""},
		{"surrounding whitespace", 404, "\n  {\"message\":\"gone\"}  \n", "", "gone", ""},
		{"unknown JSON fields", 500, `{"foo":"bar"}`, "", `{"foo":"bar"}`, ""},
		{"JSON array", 500, `["oops"]`, "", `["oops"]`, ""},
		{"invalid JSON", 500, `{"message":`, "", `{"message":`, ""},
		{"plain text", 502, "upstream connect error", "", "upstream connect error", ""},
		{"html page", 502, "<html><body>Bad Gateway</body></html>", "", "<html><body>Bad Gateway</body></html>", ""},
		{"empty body", 503, "", "", "Service Unavailable", ""},
		{"whitespace body", 404, "  \n", "", "Not Found", ""},
		{"long body is truncated", 502, longHTML, "", longHTML[:maxRawErrorMessageLength] + "…", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAPIError(tc.status, []byte(tc.body))
			if got.StatusCode != tc.status || got.Code != tc.code || got.Message != tc.message || got.Details != tc.details {
				t.Errorf("parseAPIError(%d, %q) = %+v, want status=%d code=%q message=%q details=%q",
					tc.status, tc.body, got, tc.status, tc.code, tc.message, tc.details)
			}
		})
	}

	t.Run("status_code in body is ignored", func(t *testing.T) {
		got := parseAPIError(404, []byte(`{"status_code":200,"message":"gone"}`))
		if got.StatusCode != 404 {
			t.Errorf("expected HTTP status 404, got %d", got.StatusCode)
		}
	})

	t.Run("truncation keeps valid UTF-8", func(t *testing.T) {
		got := parseAPIError(502, []byte(strings.Repeat("é", maxRawErrorMessageLength)))
		if !strings.HasSuffix(got.Message, "…") || !utf8ValidString(got.Message) {
			t.Errorf("expected valid truncated UTF-8, got %q", got.Message[len(got.Message)-8:])
		}
	})
}

func utf8ValidString(s string) bool {
	return strings.ToValidUTF8(s, "") == s
}

// TestAPIErrorDecodingAcrossServices checks that every request path (generic
// helpers with and without a result, the legacy direct requests, and a client
// without the default response middleware) decodes error bodies the same way.
func TestAPIErrorDecodingAcrossServices(t *testing.T) {
	ctx := context.Background()

	calls := []struct {
		name   string
		method string
		path   string
		call   func(c *Client) error
	}{
		{"get with result", http.MethodGet, "/locations", func(c *Client) error {
			_, err := c.Locations.Get(ctx)
			return err
		}},
		{"delete without result", http.MethodDelete, "/container-deployments/demo", func(c *Client) error {
			return c.ContainerDeployments.DeleteDeployment(ctx, "demo", 0)
		}},
		{"instance create", http.MethodPost, "/instances", func(c *Client) error {
			_, err := c.Instances.Create(ctx, CreateInstanceRequest{
				InstanceType: "1V100.6V", Image: "ubuntu-24.04", Hostname: "h", Description: "d",
			})
			return err
		}},
		{"instance action", http.MethodPut, "/instances", func(c *Client) error {
			_, err := c.Instances.Action(ctx, InstanceActionRequest{Action: ActionShutdown, ID: []string{"i-1"}})
			return err
		}},
		{"instance type availability", http.MethodGet, "/instance-availability/1V100.6V", func(c *Client) error {
			_, err := c.Instances.CheckInstanceTypeAvailability(ctx, "1V100.6V")
			return err
		}},
		{"startup script create", http.MethodPost, "/scripts", func(c *Client) error {
			_, err := c.StartupScripts.AddStartupScript(ctx, &CreateStartupScriptRequest{Name: "s", Script: "#!/bin/sh"})
			return err
		}},
		{"ssh key create", http.MethodPost, "/ssh-keys", func(c *Client) error {
			_, err := c.SSHKeys.AddSSHKey(ctx, &CreateSSHKeyRequest{Name: "k", PublicKey: "ssh-ed25519 AAAA"})
			return err
		}},
		{"volume create", http.MethodPost, "/volumes", func(c *Client) error {
			_, err := c.Volumes.CreateVolume(ctx, VolumeCreateRequest{Name: "v", Size: 10, Type: VolumeTypeNVMe})
			return err
		}},
	}

	bodies := []struct {
		name    string
		status  int
		body    string
		code    string
		message string
	}{
		{"code and message", http.StatusBadRequest, `{"code":"invalid_request","message":"name is required"}`, "invalid_request", "name is required"},
		{"error envelope", http.StatusForbidden, `{"error":"forbidden"}`, "", "forbidden"},
		{"plain text", http.StatusBadGateway, "upstream unavailable", "", "upstream unavailable"},
	}

	for _, withDefaultMiddleware := range []bool{true, false} {
		for _, tc := range calls {
			for _, b := range bodies {
				name := tc.name + "/" + b.name
				if !withDefaultMiddleware {
					name = "no response middleware/" + name
				}
				t.Run(name, func(t *testing.T) {
					mockServer := testutil.NewMockServer()
					defer mockServer.Close()
					client := NewTestClient(t, mockServer)
					if !withDefaultMiddleware {
						client.Middleware.ClearResponseMiddleware()
					}
					mockServer.SetHandler(tc.method, tc.path, func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(b.status)
						_, _ = w.Write([]byte(b.body))
					})

					err := tc.call(client)
					var apiErr *APIError
					if !errors.As(err, &apiErr) {
						t.Fatalf("expected *APIError, got %T: %v", err, err)
					}
					if apiErr.StatusCode != b.status || apiErr.Code != b.code || apiErr.Message != b.message {
						t.Errorf("got status=%d code=%q message=%q, want status=%d code=%q message=%q",
							apiErr.StatusCode, apiErr.Code, apiErr.Message, b.status, b.code, b.message)
					}
				})
			}
		}
	}
}
