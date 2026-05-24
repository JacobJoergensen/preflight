package cmd

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil is success", err: nil, want: exitSuccess},
		{name: "silent failure is findings", err: ErrSilentFailure, want: exitFindings},
		{name: "wrapped silent failure is findings", err: fmt.Errorf("check failed: %w", ErrSilentFailure), want: exitFindings},
		{name: "other error is error", err: errors.New("boom"), want: exitError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCode(tt.err); got != tt.want {
				t.Errorf("exitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
