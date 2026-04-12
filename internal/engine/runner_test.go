package engine

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"
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

func TestRunParallel(t *testing.T) {
	tests := []struct {
		name    string
		jobs    []int
		include func(int) bool
		want    []int
	}{
		{
			name:    "processes every job when count exceeds worker pool",
			jobs:    []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			include: func(int) bool { return true },
			want:    []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		},
		{
			name:    "drops results when work reports exclude",
			jobs:    []int{1, 2, 3, 4, 5, 6},
			include: func(job int) bool { return job%2 == 0 },
			want:    []int{2, 4, 6},
		},
		{
			name:    "returns nil for empty input",
			jobs:    nil,
			include: func(int) bool { return true },
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var invocations atomic.Int32

			work := func(_ context.Context, job int) (int, bool) {
				invocations.Add(1)
				return job, tt.include(job)
			}

			results := runParallel(context.Background(), tt.jobs, work)

			if len(tt.jobs) == 0 {
				if results != nil {
					t.Fatalf("expected nil result, got %v", results)
				}

				return
			}

			if int(invocations.Load()) != len(tt.jobs) {
				t.Errorf("work invoked %d times, expected %d", invocations.Load(), len(tt.jobs))
			}

			slices.Sort(results)

			if !slices.Equal(results, tt.want) {
				t.Errorf("got %v, want %v", results, tt.want)
			}
		})
	}
}

func TestRunParallelReturnsWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	jobs := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	work := func(ctx context.Context, job int) (int, bool) {
		return job, true
	}

	done := make(chan struct{})

	go func() {
		runParallel(ctx, jobs, work)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runParallel did not return after context cancellation")
	}
}
