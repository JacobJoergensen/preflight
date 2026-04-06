package exec

import (
	"context"
	"fmt"
	"io"
	goexec "os/exec"

	"github.com/JacobJoergensen/preflight/internal/manifest"
)

type DefaultStreamRunner struct{}

func (DefaultStreamRunner) RunStreaming(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
	return RunStreamingInDir(ctx, "", name, args, stdout, stderr)
}

func RunStreamingInDir(ctx context.Context, dir, name string, args []string, stdout, stderr io.Writer) error {
	if _, known := manifest.GetTool(name); !known {
		return fmt.Errorf("%w: %s", ErrCommandNotAllowed, name)
	}

	path, err := goexec.LookPath(name)

	if err != nil {
		return fmt.Errorf("command not found: %s", name)
	}

	// #nosec G204 - command validated against manifest.Tools registry
	cmd := goexec.CommandContext(ctx, path, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if dir != "" {
		cmd.Dir = dir
	}

	return cmd.Run()
}
