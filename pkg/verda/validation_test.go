package verda

import (
	"testing"
)

func TestIsLatestTag(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected bool
	}{
		// Should be detected as latest
		{"explicit latest tag", "nginx:latest", true},
		{"no tag defaults to latest", "nginx", true},
		{"registry with latest tag", "registry-1.docker.io/library/nginx:latest", true},
		{"no tag with registry", "registry-1.docker.io/library/nginx", true},

		// Should NOT be detected as latest
		{"specific version tag", "nginx:1.25.3", false},
		{"sha digest", "nginx@sha256:abc123", false},
		{"registry with version", "registry-1.docker.io/library/nginx:1.25.3", false},
		{"alpine with version", "alpine:3.19", false},
		{"python with version", "python:3.9", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLatestTag(tt.image)
			if result != tt.expected {
				t.Errorf("IsLatestTag(%q) = %v, want %v", tt.image, result, tt.expected)
			}
		})
	}
}
