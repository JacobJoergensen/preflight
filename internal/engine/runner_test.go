package engine

import "testing"

func TestIsImplicitFullSelection(t *testing.T) {
	tests := []struct {
		name   string
		only   []string
		expect bool
	}{
		{
			name:   "empty is implicit",
			only:   nil,
			expect: true,
		},
		{
			name:   "whitespace-only is implicit",
			only:   []string{"  ", ""},
			expect: true,
		},
		{
			name:   "selector set is not implicit",
			only:   []string{"js"},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isImplicitFullSelection(tt.only)

			if result != tt.expect {
				t.Errorf("got %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestSelectionIncludesEnv(t *testing.T) {
	tests := []struct {
		name   string
		only   []string
		expect bool
	}{
		{
			name:   "env present",
			only:   []string{"js", "env"},
			expect: true,
		},
		{
			name:   "ENV uppercase matches",
			only:   []string{"ENV"},
			expect: true,
		},
		{
			name:   "env with whitespace matches",
			only:   []string{"  env  "},
			expect: true,
		},
		{
			name:   "no env present",
			only:   []string{"js", "npm"},
			expect: false,
		},
		{
			name:   "empty input",
			only:   nil,
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectionIncludesEnv(tt.only)

			if result != tt.expect {
				t.Errorf("got %v, want %v", result, tt.expect)
			}
		})
	}
}
