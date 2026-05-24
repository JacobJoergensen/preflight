package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// commandWaitDelay bounds how long Wait blocks for stdio to drain after the
// process exits or the context is canceled, so a package manager that leaves a
// child holding the pipe cannot hang PreFlight after a timeout or Ctrl-C.
const commandWaitDelay = 5 * time.Second

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

// run is the single execution path behind every command PreFlight runs. It
// gates the command, resolves it, runs it writing to the given streams, and
// reports the exit code. A nonzero exit is not an error here; only gating,
// resolution, and run failures are. Callers decide whether a nonzero exit
// becomes a CommandError.
func run(ctx context.Context, dir, name string, args []string, stdout, stderr io.Writer) (Result, error) {
	if !activeGate(name) {
		return Result{ExitCode: -1}, fmt.Errorf("%w: %s", ErrCommandNotAllowed, name)
	}

	path, err := exec.LookPath(name)
	if err != nil {
		return Result{ExitCode: -1}, fmt.Errorf("command not found: %s", name)
	}

	// #nosec G204 G702 - name is gated by the command allowlist and args go straight to that binary (no shell), so neither is an injection vector
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	cmd.WaitDelay = commandWaitDelay
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	start := time.Now()
	runErr := cmd.Run()

	result := Result{
		ExitCode: cmd.ProcessState.ExitCode(),
		Duration: time.Since(start),
	}

	slog.DebugContext(ctx, "command finished", "command", name, "args", args, "exit", result.ExitCode, "duration", result.Duration)

	if runErr != nil {
		if _, exited := errors.AsType[*exec.ExitError](runErr); !exited {
			return result, runErr
		}
	}

	return result, nil
}

func Capture(ctx context.Context, dir, name string, args ...string) (Result, error) {
	var stdout, stderr bytes.Buffer

	result, err := run(ctx, dir, name, args, &stdout, &stderr)

	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	if err == nil && result.ExitCode != 0 && result.Stderr != "" {
		slog.DebugContext(ctx, "command stderr", "command", name, "stderr", result.Stderr)
	}

	return result, err
}

func Run(ctx context.Context, name string, args ...string) (Result, error) {
	result, err := Capture(ctx, "", name, args...)
	if err != nil {
		return result, err
	}

	if result.ExitCode != 0 {
		return result, commandError(name, args, result)
	}

	return result, nil
}

func commandError(name string, args []string, result Result) *CommandError {
	return &CommandError{
		Command:  name,
		Args:     args,
		ExitCode: result.ExitCode,
		Stderr:   result.Stderr,
	}
}
