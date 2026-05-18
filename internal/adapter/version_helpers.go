package adapter

import (
	"fmt"

	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type versionFeedback struct {
	Feedback        string
	EOLWarning      string
	ShouldWarnEOL   bool
	ShouldError     bool
	ShouldSuccess   bool
	ShouldWarnExtra bool
}

func buildVersionFeedback(runtime, display, installedVersion, required, versionPrefix string, satisfied bool) versionFeedback {
	feedback := fmt.Sprintf("Installed %s%s (%s ⟶ required %s)", terminal.Reset, display, installedVersion, required)

	if isEOL(runtime, versionPrefix) {
		return versionFeedback{
			Feedback:        feedback,
			EOLWarning:      fmt.Sprintf("Installed %s%s (%s ⟶ End-of-Life), consider upgrading!", terminal.Reset, display, installedVersion),
			ShouldWarnEOL:   true,
			ShouldWarnExtra: satisfied,
		}
	}

	if !satisfied {
		return versionFeedback{Feedback: feedback, ShouldError: true}
	}

	return versionFeedback{Feedback: feedback, ShouldSuccess: true}
}

func appendVersionFeedback(fb versionFeedback, errs, warns, succs []Message) ([]Message, []Message, []Message) {
	if fb.ShouldWarnEOL {
		warns = append(warns, Message{Text: fb.EOLWarning})

		if fb.ShouldWarnExtra {
			warns = append(warns, Message{Text: fb.Feedback})
		}

		return errs, warns, succs
	}

	if fb.ShouldError {
		errs = append(errs, Message{Text: fb.Feedback})
		return errs, warns, succs
	}

	if fb.ShouldSuccess {
		succs = append(succs, Message{Text: fb.Feedback})
	}

	return errs, warns, succs
}
