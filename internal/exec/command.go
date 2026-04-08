package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/manifest"
)

var ErrCommandNotAllowed = errors.New("command not in tool registry")

type CommandError struct {
	Command string
	Args    []string
	Err     error
	Stderr  string
}

func (e *CommandError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("command '%s %s' failed: %v — %s", e.Command, strings.Join(e.Args, " "), e.Err, e.Stderr)
	}

	return fmt.Sprintf("command '%s %s' failed: %v", e.Command, strings.Join(e.Args, " "), e.Err)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

func Run(ctx context.Context, name string, args ...string) (string, error) {
	if _, known := manifest.GetTool(name); !known {
		return "", fmt.Errorf("%w: %s", ErrCommandNotAllowed, name)
	}

	path, err := exec.LookPath(name)

	if err != nil {
		return "", fmt.Errorf("command not found: %s", name)
	}

	var stdout, stderr bytes.Buffer

	// #nosec G204 - command validated against manifest.Tools registry
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(stdout.String()), &CommandError{
			Command: name,
			Args:    args,
			Err:     err,
			Stderr:  strings.TrimSpace(stderr.String()),
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}
