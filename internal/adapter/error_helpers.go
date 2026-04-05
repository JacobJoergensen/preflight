package adapter

import (
	"errors"

	"github.com/JacobJoergensen/preflight/internal/exec"
)

func formatExecFailure(label string, err error) string {
	if err == nil {
		return label
	}

	detail := err.Error()

	if cmdErr, ok := errors.AsType[*exec.CommandError](err); ok && cmdErr.Stderr != "" {
		detail = cmdErr.Stderr
	}

	if len(detail) > 400 {
		detail = detail[:397] + "…"
	}

	return label + ": " + detail
}
