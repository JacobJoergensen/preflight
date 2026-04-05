package engine

import (
	"testing"
)

func TestIsImplicitFullSelection(t *testing.T) {
	tests := []struct {
		name      string
		scopes    []string
		selectors []string
		expect    bool
	}{
		{
			name:      "both empty is implicit",
			scopes:    nil,
			selectors: nil,
			expect:    true,
		},
		{
			name:      "whitespace-only is implicit",
			scopes:    []string{"  ", ""},
			selectors: []string{""},
			expect:    true,
		},
		{
			name:      "scope set is not implicit",
			scopes:    []string{"js"},
			selectors: nil,
			expect:    false,
		},
		{
			name:      "selector set is not implicit",
			scopes:    nil,
			selectors: []string{"npm"},
			expect:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isImplicitFullSelection(tt.scopes, tt.selectors)

			if result != tt.expect {
				t.Errorf("got %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestSelectionIncludesEnv(t *testing.T) {
	tests := []struct {
		name      string
		scopes    []string
		selectors []string
		expect    bool
	}{
		{
			name:   "env in scopes",
			scopes: []string{"js", "env"},
			expect: true,
		},
		{
			name:      "env in selectors",
			selectors: []string{"npm", "env"},
			expect:    true,
		},
		{
			name:   "ENV uppercase matches",
			scopes: []string{"ENV"},
			expect: true,
		},
		{
			name:   "env with whitespace matches",
			scopes: []string{"  env  "},
			expect: true,
		},
		{
			name:      "no env present",
			scopes:    []string{"js"},
			selectors: []string{"npm"},
			expect:    false,
		},
		{
			name:   "empty inputs",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectionIncludesEnv(tt.scopes, tt.selectors)

			if result != tt.expect {
				t.Errorf("got %v, want %v", result, tt.expect)
			}
		})
	}
}

func TestParallelWorkerCount(t *testing.T) {
	tests := []struct {
		name     string
		jobCount int
		wantMin  int
		wantMax  int
	}{
		{
			name:     "zero jobs returns 1",
			jobCount: 0,
			wantMin:  1,
			wantMax:  1,
		},
		{
			name:     "negative jobs returns 1",
			jobCount: -5,
			wantMin:  1,
			wantMax:  1,
		},
		{
			name:     "single job returns 1",
			jobCount: 1,
			wantMin:  1,
			wantMax:  1,
		},
		{
			name:     "many jobs capped at 8",
			jobCount: 100,
			wantMin:  1,
			wantMax:  8,
		},
		{
			name:     "few jobs limited by job count",
			jobCount: 3,
			wantMin:  1,
			wantMax:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parallelWorkerCount(tt.jobCount)

			if result < tt.wantMin || result > tt.wantMax {
				t.Errorf("got %d, want between %d and %d", result, tt.wantMin, tt.wantMax)
			}
		})
	}
}
