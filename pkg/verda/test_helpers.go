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

import "github.com/verda-cloud/verdacloud-sdk-go/pkg/verda/testutil"

func NewTestClient(mockServer *testutil.MockServer) *Client {
	config := testutil.NewTestClientConfig(mockServer)
	client, _ := NewClient(
		WithBaseURL(config.BaseURL),
		WithClientID(config.ClientID),
		WithClientSecret(config.ClientSecret),
	)
	return client
}

func NewTestClientWithUserAgent(mockServer *testutil.MockServer, userAgent string) *Client {
	config := testutil.NewTestClientConfig(mockServer)
	client, _ := NewClient(
		WithBaseURL(config.BaseURL),
		WithClientID(config.ClientID),
		WithClientSecret(config.ClientSecret),
		WithUserAgent(userAgent),
	)
	return client
}
