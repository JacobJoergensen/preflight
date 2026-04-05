package engine

import (
	"slices"
	"strings"
	"testing"
)

func TestSelect(t *testing.T) {
	tests := []struct {
		name       string
		input      SelectInput
		wantIDs    []string
		wantMode   Mode
		wantError  bool
		errContain string
	}{
		{
			name:     "empty input returns all adapters",
			input:    SelectInput{Mode: ModeCheck},
			wantMode: ModeCheck,
		},
		{
			name:      "cannot use both scopes and selectors",
			input:     SelectInput{Scopes: []string{"js"}, Selectors: []string{"npm"}, Mode: ModeCheck},
			wantError: true,
		},
		{
			name:       "unknown scope returns error",
			input:      SelectInput{Scopes: []string{"invalid"}, Mode: ModeCheck},
			wantError:  true,
			errContain: "unknown scope",
		},
		{
			name:     "valid scope selects adapter",
			input:    SelectInput{Scopes: []string{"js"}, Mode: ModeCheck},
			wantIDs:  []string{"js"},
			wantMode: ModeCheck,
		},
		{
			name:     "selector npm resolves to js adapter",
			input:    SelectInput{Selectors: []string{"npm"}, Mode: ModeCheck},
			wantIDs:  []string{"js"},
			wantMode: ModeCheck,
		},
		{
			name:     "fix mode with selector preserves selector",
			input:    SelectInput{Selectors: []string{"yarn"}, Mode: ModeFix},
			wantIDs:  []string{"js"},
			wantMode: ModeFix,
		},
		{
			name:       "fix mode with unknown selector returns error",
			input:      SelectInput{Selectors: []string{"unknown"}, Mode: ModeFix},
			wantError:  true,
			errContain: "unknown selector",
		},
		{
			name:     "whitespace-only inputs treated as empty",
			input:    SelectInput{Scopes: []string{"  ", ""}, Mode: ModeCheck},
			wantMode: ModeCheck,
		},
		{
			name:     "input is case-insensitive",
			input:    SelectInput{Scopes: []string{"JS"}, Mode: ModeCheck},
			wantIDs:  []string{"js"},
			wantMode: ModeCheck,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Select(tt.input)

			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContain)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.RequestedMode != tt.wantMode {
				t.Errorf("mode = %q, want %q", result.RequestedMode, tt.wantMode)
			}

			if tt.wantIDs != nil && !slices.Equal(result.AdapterIDs, tt.wantIDs) {
				t.Errorf("adapterIDs = %v, want %v", result.AdapterIDs, tt.wantIDs)
			}
		})
	}
}

func TestNormalizeNames(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{
			name:   "trims whitespace and lowercases",
			input:  []string{"  JS  ", "NPM", " Composer "},
			expect: []string{"js", "npm", "composer"},
		},
		{
			name:   "filters empty strings",
			input:  []string{"js", "", "  ", "npm"},
			expect: []string{"js", "npm"},
		},
		{
			name:   "nil input returns empty slice",
			input:  nil,
			expect: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeNames(tt.input)

			if !slices.Equal(result, tt.expect) {
				t.Errorf("got %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestDedupePreserveOrder(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{
			name:   "removes duplicates preserving first occurrence",
			input:  []string{"a", "b", "a", "c", "b"},
			expect: []string{"a", "b", "c"},
		},
		{
			name:   "no duplicates returns same order",
			input:  []string{"x", "y", "z"},
			expect: []string{"x", "y", "z"},
		},
		{
			name:   "empty input returns empty",
			input:  []string{},
			expect: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dedupePreserveOrder(tt.input)

			if !slices.Equal(result, tt.expect) {
				t.Errorf("got %v, want %v", result, tt.expect)
			}
		})
	}
}
