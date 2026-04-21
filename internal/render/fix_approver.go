package render

import (
	"io"

	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type TTYFixApprover struct {
	in          io.Reader
	out         io.Writer
	allApproved bool
}

func NewTTYFixApprover(in io.Reader, out io.Writer) *TTYFixApprover {
	return &TTYFixApprover{in: in, out: out}
}

func (a *TTYFixApprover) Approve(candidate engine.FixCandidate) (engine.FixDecision, error) {
	if a.allApproved {
		return engine.FixApply, nil
	}

	renderFixPromptBlock(terminal.NewOutputWriterFor(a.out), candidate)

	answer, err := terminal.Confirm(a.in, a.out, "Apply fix?", "")

	if err != nil {
		return 0, err
	}

	switch answer {
	case terminal.AnswerYes:
		return engine.FixApply, nil
	case terminal.AnswerNo:
		return engine.FixSkip, nil
	case terminal.AnswerAll:
		a.allApproved = true
		return engine.FixApply, nil
	case terminal.AnswerQuit:
		return engine.FixAbort, nil
	default:
		return engine.FixSkip, nil
	}
}

func renderFixPromptBlock(ow *terminal.OutputWriter, candidate engine.FixCandidate) {
	gutter := terminal.Gray + "┃" + terminal.Reset

	ow.PrintNewLines(1)
	ow.Printf("%s %s%s%s\n", gutter, terminal.Bold, candidate.DisplayName, terminal.Reset)

	if candidate.Command != "" {
		ow.Printf("%s   %s→%s %s\n", gutter, terminal.Cyan, terminal.Reset, candidate.Command)
	}

	if candidate.Summary != "" {
		ow.Printf("%s   %s%s%s\n", gutter, terminal.Dim, candidate.Summary, terminal.Reset)
	}

	ow.Println(gutter)
}
