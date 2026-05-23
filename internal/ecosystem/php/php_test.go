package php

import "testing"

func TestFindPdoAlternative(t *testing.T) {
	tests := []struct {
		name    string
		ext     string
		sources map[string]string
		want    string
	}{
		{
			name:    "returns an installed pdo driver for pdo",
			ext:     "pdo",
			sources: map[string]string{"pdo_mysql": "core"},
			want:    "pdo_mysql",
		},
		{
			name:    "returns empty when no pdo driver is installed",
			ext:     "pdo",
			sources: map[string]string{"json": "core"},
			want:    "",
		},
		{
			name:    "returns empty for a non-split extension",
			ext:     "json",
			sources: map[string]string{"pdo_mysql": "core"},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findPdoAlternative(tt.ext, tt.sources); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
