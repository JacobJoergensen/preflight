package exec

import (
	"context"
	"io"
)

type DefaultStreamRunner struct{}

func (DefaultStreamRunner) RunStreaming(ctx context.Context, name string, args []string, stdout, stderr io.Writer) (Result, error) {
	return RunStreamingInDir(ctx, "", name, args, stdout, stderr)
}

func RunStreamingInDir(ctx context.Context, dir, name string, args []string, stdout, stderr io.Writer) (Result, error) {
	result, err := run(ctx, dir, name, args, stdout, stderr)
	if err != nil {
		return result, err
	}

	if result.ExitCode != 0 {
		return result, commandError(name, args, result)
	}

	return result, nil
}
