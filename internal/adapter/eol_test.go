package adapter

import "testing"

func TestIsEOL(t *testing.T) {
	tests := []struct {
		name          string
		runtime       string
		versionPrefix string
		want          bool
	}{
		// PHP EOL versions
		{"php 5.6 is EOL", "php", "5.6", true},
		{"php 7.4 is EOL", "php", "7.4", true},
		{"php 8.0 is EOL", "php", "8.0", true},
		{"php 8.1 is EOL", "php", "8.1", true},
		{"php 8.2 is not EOL", "php", "8.2", false},
		{"php 8.3 is not EOL", "php", "8.3", false},

		// Node EOL versions
		{"node 16 is EOL", "node", "16", true},
		{"node 18 is EOL", "node", "18", true},
		{"node 19 is EOL", "node", "19", true},
		{"node 20 is not EOL", "node", "20", false},
		{"node 22 is not EOL", "node", "22", false},

		// Go EOL versions
		{"go 1.20 is EOL", "go", "1.20", true},
		{"go 1.22 is EOL", "go", "1.22", true},
		{"go 1.24 is EOL", "go", "1.24", true},
		{"go 1.25 is not EOL", "go", "1.25", false},
		{"go 1.26 is not EOL", "go", "1.26", false},

		// Case insensitive runtime
		{"PHP uppercase", "PHP", "8.0", true},
		{"Node uppercase", "NODE", "18", true},
		{"Go uppercase", "GO", "1.22", true},

		// Unknown runtime
		{"unknown runtime returns false", "ruby", "3.0", false},
		{"empty runtime returns false", "", "1.0", false},

		// Unknown version prefix
		{"unknown version prefix returns false", "php", "9.9", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEOL(tt.runtime, tt.versionPrefix)

			if got != tt.want {
				t.Errorf("isEOL(%q, %q) = %v, want %v",
					tt.runtime, tt.versionPrefix, got, tt.want)
			}
		})
	}
}
