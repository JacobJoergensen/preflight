package exec

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	goexec "os/exec"
	"time"
)

type DefaultStreamRunner struct{}

func (DefaultStreamRunner) RunStreaming(ctx context.Context, name string, args []string, stdout, stderr io.Writer) (Result, error) {
	return RunStreamingInDir(ctx, "", name, args, stdout, stderr)
}

func RunStreamingInDir(ctx context.Context, dir, name string, args []string, stdout, stderr io.Writer) (Result, error) {
	if !activeGate(name) {
		return Result{ExitCode: -1}, fmt.Errorf("%w: %s", ErrCommandNotAllowed, name)
	}

	path, err := goexec.LookPath(name)
	if err != nil {
		return Result{ExitCode: -1}, fmt.Errorf("command not found: %s", name)
	}

	// #nosec G204 - command validated against the active command gate
	cmd := goexec.CommandContext(ctx, path, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if dir != "" {
		cmd.Dir = dir
	}

	start := time.Now()
	runErr := cmd.Run()

	result := Result{
		ExitCode: cmd.ProcessState.ExitCode(),
		Duration: time.Since(start),
	}

	slog.DebugContext(ctx, "command finished", "command", name, "args", args, "exit", result.ExitCode, "duration", result.Duration)

	return result, runErr
}
