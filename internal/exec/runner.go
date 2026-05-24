package exec

import (
	"context"
	"io"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
}

type StreamRunner interface {
	RunStreaming(ctx context.Context, name string, args []string, stdout, stderr io.Writer) (Result, error)
}

type DefaultRunner struct{}

func (DefaultRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	return Run(ctx, name, args...)
}
