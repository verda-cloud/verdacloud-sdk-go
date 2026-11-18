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
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"
)

func TestPathSegment(t *testing.T) {
	cases := []struct {
		value   string
		want    string
		wantErr string
	}{
		{value: "", wantErr: "id must not be empty"},
		{value: ".", wantErr: `id "." is not a valid path segment`},
		{value: "..", wantErr: `id ".." is not a valid path segment`},
		{value: "pn_123", want: "pn_123"},
		{value: "a/b", want: "a%2Fb"},
		{value: "a?b", want: "a%3Fb"},
		{value: "a#b", want: "a%23b"},
		{value: "a%b", want: "a%25b"},
		{value: "a%2Fb", want: "a%252Fb"},
		{value: "...", want: "..."},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			got, err := pathSegment("id", tc.value)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("expected error %q, got %q, %v", tc.wantErr, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

// sentRequest is one API request as it left the client
type sentRequest struct {
	method, path, query string
}

// recordingTransport records every API request the client sends, except the
// OAuth token request
type recordingTransport struct {
	mu   sync.Mutex
	sent []sentRequest
}

func (rt *recordingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Path != "/oauth2/token" {
		rt.mu.Lock()
		rt.sent = append(rt.sent, sentRequest{r.Method, r.URL.EscapedPath(), r.URL.RawQuery})
		rt.mu.Unlock()
	}
	return http.DefaultTransport.RoundTrip(r)
}

func (rt *recordingTransport) requests() []sentRequest {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return append([]sentRequest(nil), rt.sent...)
}

func newRecordingClient(t *testing.T) (*Client, *recordingTransport) {
	t.Helper()

	srv := testutil.NewMockServer()
	t.Cleanup(srv.Close)

	rt := &recordingTransport{}
	client := NewTestClient(t, srv)
	client.HTTPClient.Transport = rt
	return client, rt
}

// pathCase is one service method that puts caller-supplied values into the
// request path. call receives one value per %s in path, in order.
type pathCase struct {
	name   string
	method string
	path   string // escaped path, one %s per caller-supplied segment
	query  string // expected raw query; empty means none
	call   func(ctx context.Context, c *Client, seg []string) error
}

func errOnly[T any](_ T, err error) error { return err }

func pathCases() []pathCase {
	isPermanent, force, spot := true, true, true
	return []pathCase{
		// Private networks
		{"PrivateNetworks.GetByID", http.MethodGet, "/private-networks/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.PrivateNetworks.GetByID(ctx, s[0]))
			}},
		{"PrivateNetworks.Update", http.MethodPatch, "/private-networks/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.PrivateNetworks.Update(ctx, s[0], UpdatePrivateNetworkRequest{}))
			}},
		{"PrivateNetworks.Delete", http.MethodDelete, "/private-networks/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return c.PrivateNetworks.Delete(ctx, s[0])
			}},
		{"PrivateNetworks.GetWorkloads", http.MethodGet, "/private-networks/%s/workloads", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.PrivateNetworks.GetWorkloads(ctx, s[0]))
			}},
		{"PrivateNetworks.GetSubnetWorkloads", http.MethodGet, "/private-networks/%s/subnets/%s/workloads", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.PrivateNetworks.GetSubnetWorkloads(ctx, s[0], s[1]))
			}},
		{"PrivateNetworks.CreateSubnet", http.MethodPost, "/private-networks/%s/subnets", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.PrivateNetworks.CreateSubnet(ctx, s[0], CreateSubnetRequest{
					LocationCode: "FIN-01", Name: "workers", CIDR: "10.0.0.0/24",
				}))
			}},
		{"PrivateNetworks.GetSubnets", http.MethodGet, "/private-networks/%s/subnets", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.PrivateNetworks.GetSubnets(ctx, s[0]))
			}},
		{"PrivateNetworks.GetSubnetByID", http.MethodGet, "/private-networks/%s/subnets/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.PrivateNetworks.GetSubnetByID(ctx, s[0], s[1]))
			}},
		{"PrivateNetworks.UpdateSubnet", http.MethodPatch, "/private-networks/%s/subnets/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.PrivateNetworks.UpdateSubnet(ctx, s[0], s[1], UpdateSubnetRequest{}))
			}},
		{"PrivateNetworks.DeleteSubnet", http.MethodDelete, "/private-networks/%s/subnets/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return c.PrivateNetworks.DeleteSubnet(ctx, s[0], s[1])
			}},
		{"PrivateNetworks.CreateRoute", http.MethodPost, "/private-networks/%s/subnets/%s/routes", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.PrivateNetworks.CreateRoute(ctx, s[0], s[1], CreateRouteRequest{
					Destination: "0.0.0.0/0", InstanceID: "inst_123",
				}))
			}},
		{"PrivateNetworks.GetRoutes", http.MethodGet, "/private-networks/%s/subnets/%s/routes", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.PrivateNetworks.GetRoutes(ctx, s[0], s[1]))
			}},
		{"PrivateNetworks.DeleteRoute", http.MethodDelete, "/private-networks/%s/subnets/%s/routes/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return c.PrivateNetworks.DeleteRoute(ctx, s[0], s[1], s[2])
			}},

		// Clusters
		{"Clusters.GetByID", http.MethodGet, "/clusters/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.Clusters.GetByID(ctx, s[0]))
			}},
		{"Clusters.CheckClusterTypeAvailability", http.MethodGet, "/cluster-availability/%s", "location_code=FIN-01",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.Clusters.CheckClusterTypeAvailability(ctx, s[0], "FIN-01"))
			}},

		// Instances, instance types and availability
		{"Instances.GetByID", http.MethodGet, "/instances/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.Instances.GetByID(ctx, s[0]))
			}},
		{"Instances.CheckInstanceTypeAvailability", http.MethodGet, "/instance-availability/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.Instances.CheckInstanceTypeAvailability(ctx, s[0]))
			}},
		{"InstanceTypes.GetByInstanceType", http.MethodGet, "/instance-types/%s", "currency=eur&is_spot=true&location_code=FIN-01",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.InstanceTypes.GetByInstanceType(ctx, s[0], spot, "FIN-01", "eur"))
			}},
		{"InstanceAvailability.GetInstanceTypeAvailability", http.MethodGet, "/instance-availability/%s", "is_spot=true&location_code=FIN-01",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.InstanceAvailability.GetInstanceTypeAvailability(ctx, s[0], spot, "FIN-01"))
			}},

		// Volumes
		{"Volumes.GetVolume", http.MethodGet, "/volumes/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.Volumes.GetVolume(ctx, s[0]))
			}},
		{"Volumes.DeleteVolume", http.MethodDelete, "/volumes/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return c.Volumes.DeleteVolume(ctx, s[0], isPermanent)
			}},

		// SSH keys and startup scripts
		{"SSHKeys.GetSSHKeyByID", http.MethodGet, "/ssh-keys/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.SSHKeys.GetSSHKeyByID(ctx, s[0]))
			}},
		{"SSHKeys.DeleteSSHKey", http.MethodDelete, "/ssh-keys/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return c.SSHKeys.DeleteSSHKey(ctx, s[0])
			}},
		{"StartupScripts.GetStartupScriptByID", http.MethodGet, "/scripts/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.StartupScripts.GetStartupScriptByID(ctx, s[0]))
			}},
		{"StartupScripts.DeleteStartupScript", http.MethodDelete, "/scripts/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return c.StartupScripts.DeleteStartupScript(ctx, s[0])
			}},

		// Serverless jobs
		{"ServerlessJobs.GetJobDeploymentByName", http.MethodGet, "/job-deployments/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.ServerlessJobs.GetJobDeploymentByName(ctx, s[0]))
			}},
		{"ServerlessJobs.UpdateJobDeployment", http.MethodPatch, "/job-deployments/%s", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.ServerlessJobs.UpdateJobDeployment(ctx, s[0], &UpdateJobDeploymentRequest{}))
			}},
		{"ServerlessJobs.DeleteJobDeployment", http.MethodDelete, "/job-deployments/%s", "timeout=1000",
			func(ctx context.Context, c *Client, s []string) error {
				return c.ServerlessJobs.DeleteJobDeployment(ctx, s[0], 1000)
			}},
		{"ServerlessJobs.GetJobDeploymentScaling", http.MethodGet, "/job-deployments/%s/scaling", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.ServerlessJobs.GetJobDeploymentScaling(ctx, s[0]))
			}},
		{"ServerlessJobs.PurgeJobDeploymentQueue", http.MethodPost, "/job-deployments/%s/purge-queue", "",
			func(ctx context.Context, c *Client, s []string) error {
				return c.ServerlessJobs.PurgeJobDeploymentQueue(ctx, s[0])
			}},
		{"ServerlessJobs.PauseJobDeployment", http.MethodPost, "/job-deployments/%s/pause", "",
			func(ctx context.Context, c *Client, s []string) error {
				return c.ServerlessJobs.PauseJobDeployment(ctx, s[0])
			}},
		{"ServerlessJobs.ResumeJobDeployment", http.MethodPost, "/job-deployments/%s/resume", "",
			func(ctx context.Context, c *Client, s []string) error {
				return c.ServerlessJobs.ResumeJobDeployment(ctx, s[0])
			}},
		{"ServerlessJobs.GetJobDeploymentStatus", http.MethodGet, "/job-deployments/%s/status", "",
			func(ctx context.Context, c *Client, s []string) error {
				return errOnly(c.ServerlessJobs.GetJobDeploymentStatus(ctx, s[0]))
			}},

		// Container deployment secrets and registry credentials
		{"ContainerDeployments.DeleteSecret", http.MethodDelete, "/secrets/%s", "force=true",
			func(ctx context.Context, c *Client, s []string) error {
				return c.ContainerDeployments.DeleteSecret(ctx, s[0], force)
			}},
		{"ContainerDeployments.DeleteFileSecret", http.MethodDelete, "/file-secrets/%s", "force=true",
			func(ctx context.Context, c *Client, s []string) error {
				return c.ContainerDeployments.DeleteFileSecret(ctx, s[0], force)
			}},
		{"ContainerDeployments.DeleteRegistryCredentials", http.MethodDelete, "/container-registry-credentials/%s", "force=true",
			func(ctx context.Context, c *Client, s []string) error {
				return c.ContainerDeployments.DeleteRegistryCredentials(ctx, s[0], force)
			}},
	}
}

// validSegments returns n distinct, ordinary path values
func validSegments(n int) []string {
	segs := make([]string, n)
	for i := range segs {
		segs[i] = fmt.Sprintf("id%d", i+1)
	}
	return segs
}

func expectedPath(format string, segs []string) string {
	args := make([]any, len(segs))
	for i, s := range segs {
		args[i] = s
	}
	return fmt.Sprintf(format, args...)
}

// TestRequestPaths_EscapeSegments checks, for every caller-supplied path
// segment of every method, that a value with reserved characters arrives as
// one escaped segment and the query string is unchanged.
func TestRequestPaths_EscapeSegments(t *testing.T) {
	const (
		raw     = "a/b?c#d%e%2F"
		escaped = "a%2Fb%3Fc%23d%25e%252F"
	)
	ctx := context.Background()

	for _, tc := range pathCases() {
		n := strings.Count(tc.path, "%s")
		for pos := range n {
			t.Run(fmt.Sprintf("%s/segment_%d", tc.name, pos+1), func(t *testing.T) {
				client, rt := newRecordingClient(t)

				segs := validSegments(n)
				want := append([]string(nil), segs...)
				segs[pos], want[pos] = raw, escaped

				// The response does not matter here, only the request
				_ = tc.call(ctx, client, segs)

				sent := rt.requests()
				if len(sent) == 0 {
					t.Fatal("expected a request, got none")
				}
				got := sent[0]
				if got.method != tc.method {
					t.Errorf("expected method %s, got %s", tc.method, got.method)
				}
				if wantPath := expectedPath(tc.path, want); got.path != wantPath {
					t.Errorf("expected path %q, got %q", wantPath, got.path)
				}
				if got.query != tc.query {
					t.Errorf("expected query %q, got %q", tc.query, got.query)
				}
			})
		}
	}
}

// TestRequestPaths_RejectInvalidSegments checks, for every caller-supplied
// path segment of every method, that an empty value, "." or ".." returns an
// error before any request is sent. Unescaped, these values would call a
// different endpoint: an empty ID turns a get-by-ID into a list call.
func TestRequestPaths_RejectInvalidSegments(t *testing.T) {
	ctx := context.Background()

	for _, tc := range pathCases() {
		n := strings.Count(tc.path, "%s")
		for pos := range n {
			for _, bad := range []string{"", ".", ".."} {
				t.Run(fmt.Sprintf("%s/segment_%d/%q", tc.name, pos+1, bad), func(t *testing.T) {
					client, rt := newRecordingClient(t)

					segs := validSegments(n)
					segs[pos] = bad

					if err := tc.call(ctx, client, segs); err == nil {
						t.Error("expected an error, got nil")
					}
					if sent := rt.requests(); len(sent) != 0 {
						t.Errorf("expected no request, got %+v", sent)
					}
				})
			}
		}
	}
}

// TestRequestPaths_KeepExistingEmptyErrors pins the error text of methods that
// rejected an empty value before path validation was added
func TestRequestPaths_KeepExistingEmptyErrors(t *testing.T) {
	ctx := context.Background()
	client, _ := newRecordingClient(t)

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"ServerlessJobs.UpdateJobDeployment",
			errOnly(client.ServerlessJobs.UpdateJobDeployment(ctx, "", &UpdateJobDeploymentRequest{})),
			"jobName is required"},
		{"ServerlessJobs.GetJobDeploymentScaling",
			errOnly(client.ServerlessJobs.GetJobDeploymentScaling(ctx, "")),
			"jobName is required"},
	}
	for _, tc := range cases {
		if tc.err == nil || tc.err.Error() != tc.want {
			t.Errorf("%s: expected error %q, got %v", tc.name, tc.want, tc.err)
		}
	}
}
