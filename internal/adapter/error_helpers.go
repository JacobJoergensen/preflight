package adapter

import (
	"errors"

	"github.com/JacobJoergensen/preflight/internal/exec"
)

const execErrorDetailMax = 400

func formatExecFailure(label string, err error) string {
	if err == nil {
		return label
	}

	detail := err.Error()

	if cmdErr, ok := errors.AsType[*exec.CommandError](err); ok && cmdErr.Stderr != "" {
		detail = cmdErr.Stderr
	}

	if len(detail) > execErrorDetailMax {
		detail = detail[:execErrorDetailMax-len("…")] + "…"
	}

	return label + ": " + detail
}
