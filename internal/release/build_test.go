package release

import "testing"

func TestReleaseVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"tagged with v prefix", "v2.0.0", "2.0.0"},
		{"tagged without prefix", "2.0.0", "2.0.0"},
		{"devel build", "(devel)", ""},
		{"pseudo-version", "v0.0.0-20260524060539-7502eb5ff0b2+dirty", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := releaseVersion(tt.in); got != tt.want {
				t.Errorf("releaseVersion(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
