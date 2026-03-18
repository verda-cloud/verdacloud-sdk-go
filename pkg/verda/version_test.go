package verda

import (
	"strings"
	"testing"
)

func TestSDKVersion(t *testing.T) {
	version := SDKVersion()

	if version == "" {
		t.Error("SDKVersion() should return a non-empty string")
	}

	// Version should be in semver format (e.g., "1.2.0")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		t.Errorf("SDKVersion() should return semver format (x.y.z), got %q", version)
	}
}

func TestDefaultUserAgent(t *testing.T) {
	ua := DefaultUserAgent()

	if ua == "" {
		t.Error("DefaultUserAgent() should return a non-empty string")
	}

	// Should contain SDK name and version
	if !strings.Contains(ua, "verdacloud-sdk-go/") {
		t.Errorf("DefaultUserAgent() should contain 'verdacloud-sdk-go/', got %q", ua)
	}

	// Should contain the version
	version := SDKVersion()
	if !strings.Contains(ua, version) {
		t.Errorf("DefaultUserAgent() should contain version %q, got %q", version, ua)
	}
}

func TestBuildUserAgent(t *testing.T) {
	tests := []struct {
		name           string
		customUA       string
		expectedPrefix string
		expectedSuffix string
	}{
		{
			name:           "empty custom user agent returns default",
			customUA:       "",
			expectedPrefix: "verdacloud-sdk-go/",
			expectedSuffix: "",
		},
		{
			name:           "custom user agent is prepended",
			customUA:       "my-terraform/1.4.2",
			expectedPrefix: "my-terraform/1.4.2",
			expectedSuffix: "verdacloud-sdk-go/",
		},
		{
			name:           "complex custom user agent",
			customUA:       "my-product-terraform-provider/1.4.2 terraform/1.6.5",
			expectedPrefix: "my-product-terraform-provider/1.4.2 terraform/1.6.5",
			expectedSuffix: "verdacloud-sdk-go/",
		},
		{
			name:           "CRLF characters are stripped",
			customUA:       "my-app/1.0\r\nEvil-Header: injected",
			expectedPrefix: "my-app/1.0Evil-Header: injected",
			expectedSuffix: "verdacloud-sdk-go/",
		},
		{
			name:           "null bytes and control chars are stripped",
			customUA:       "my-app/1.0\x00\x01\x1F",
			expectedPrefix: "my-app/1.0",
			expectedSuffix: "verdacloud-sdk-go/",
		},
		{
			name:           "only control chars returns default",
			customUA:       "\r\n\x00",
			expectedPrefix: "verdacloud-sdk-go/",
			expectedSuffix: "",
		},
		{
			name:           "whitespace-only returns default",
			customUA:       "   ",
			expectedPrefix: "verdacloud-sdk-go/",
			expectedSuffix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildUserAgent(tt.customUA)

			if !strings.HasPrefix(result, tt.expectedPrefix) {
				t.Errorf("BuildUserAgent(%q) should start with %q, got %q",
					tt.customUA, tt.expectedPrefix, result)
			}

			if tt.expectedSuffix != "" && !strings.Contains(result, tt.expectedSuffix) {
				t.Errorf("BuildUserAgent(%q) should contain %q, got %q",
					tt.customUA, tt.expectedSuffix, result)
			}

			// Should always contain SDK identifier
			if !strings.Contains(result, "verdacloud-sdk-go/") {
				t.Errorf("BuildUserAgent(%q) should always contain 'verdacloud-sdk-go/', got %q",
					tt.customUA, result)
			}
		})
	}
}

func TestSanitizeUserAgent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"clean input unchanged", "my-app/1.0", "my-app/1.0"},
		{"strips CR LF", "a\r\nb", "ab"},
		{"strips null byte", "a\x00b", "ab"},
		{"strips all control chars", "\x01\x02\x1F\x7F", ""},
		{"strips DEL char", "a\x7Fb", "ab"},
		{"trims surrounding whitespace", "  my-app/1.0  ", "my-app/1.0"},
		{"preserves internal spaces", "my-app/1.0 terraform/1.6", "my-app/1.0 terraform/1.6"},
		{"caps at 256 characters", strings.Repeat("a", 300), strings.Repeat("a", 256)},
		{"empty string stays empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeUserAgent(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeUserAgent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
