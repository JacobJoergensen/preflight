package render

import "testing"

func TestDepNoun(t *testing.T) {
	tests := []struct {
		name   string
		sample string
		count  int
		want   string
	}{
		{"go modules plural", "Installed module github.com/spf13/cobra (1.8.0)", 4, "modules"},
		{"single module stays singular", "Installed module github.com/spf13/cobra (1.8.0)", 1, "module"},
		{"composer dependency pluralizes to ies", "Installed dependency vendor/pkg (1.0)", 12, "dependencies"},
		{"single dependency stays singular", "Installed dependency vendor/pkg (1.0)", 1, "dependency"},
		{"rust crates", "Installed crate serde (1.0)", 2, "crates"},
		{"unrecognized line falls back to dependency", "totally different line", 3, "dependencies"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := depNoun(tt.sample, tt.count); got != tt.want {
				t.Errorf("depNoun(%q, %d) = %q, want %q", tt.sample, tt.count, got, tt.want)
			}
		})
	}
}
