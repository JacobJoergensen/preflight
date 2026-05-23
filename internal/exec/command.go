package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

var ErrCommandNotAllowed = errors.New("command not allowed")

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

type CommandError struct {
	Command  string
	Args     []string
	ExitCode int
	Stderr   string
}

func (e *CommandError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("command '%s %s' exited with code %d: %s", e.Command, strings.Join(e.Args, " "), e.ExitCode, e.Stderr)
	}

	return fmt.Sprintf("command '%s %s' exited with code %d", e.Command, strings.Join(e.Args, " "), e.ExitCode)
}

type Gate func(name string) bool

var activeGate Gate = denyAll

func denyAll(string) bool { return false }

func SetGate(gate Gate) {
	if gate != nil {
		activeGate = gate
	}
}

func Capture(ctx context.Context, gate Gate, dir, name string, args ...string) (Result, error) {
	if gate == nil || !gate(name) {
		return Result{ExitCode: -1}, fmt.Errorf("%w: %s", ErrCommandNotAllowed, name)
	}

	path, err := exec.LookPath(name)
	if err != nil {
		return Result{ExitCode: -1}, fmt.Errorf("command not found: %s", name)
	}

	var stdout, stderr bytes.Buffer

	// #nosec G204 - command authorized by the provided gate
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if dir != "" {
		cmd.Dir = dir
	}

	start := time.Now()
	runErr := cmd.Run()

	result := Result{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: cmd.ProcessState.ExitCode(),
		Duration: time.Since(start),
	}

	attrs := []any{"command", name, "args", args, "exit", result.ExitCode, "duration", result.Duration}

	if result.ExitCode != 0 && result.Stderr != "" {
		attrs = append(attrs, "stderr", result.Stderr)
	}

	slog.DebugContext(ctx, "command finished", attrs...)

	if runErr != nil {
		if _, exited := errors.AsType[*exec.ExitError](runErr); !exited {
			return result, runErr
		}
	}

	return result, nil
}

func Run(ctx context.Context, name string, args ...string) (Result, error) {
	result, err := Capture(ctx, activeGate, "", name, args...)
	if err != nil {
		return result, err
	}

	if result.ExitCode != 0 {
		return result, &CommandError{
			Command:  name,
			Args:     args,
			ExitCode: result.ExitCode,
			Stderr:   result.Stderr,
		}
	}

	return result, nil
}
