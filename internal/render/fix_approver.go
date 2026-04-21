package render

import (
	"io"

	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type TTYFixApprover struct {
	in                 io.Reader
	out                io.Writer
	allApproved        bool
	autoApproveProject string
	lastPrintedProject string
	lastPromptProject  string
	haveSeenCandidate  bool
}

func NewTTYFixApprover(in io.Reader, out io.Writer) *TTYFixApprover {
	return &TTYFixApprover{in: in, out: out}
}

func (a *TTYFixApprover) Approve(candidate engine.FixCandidate) (engine.FixDecision, error) {
	if a.allApproved {
		return engine.FixApply, nil
	}

	a.resetProjectScopeIfChanged(candidate.Project)

	if a.autoApproveProject != "" && a.autoApproveProject == candidate.Project {
		return engine.FixApply, nil
	}

	writer := terminal.NewOutputWriterFor(a.out)

	a.renderProjectHeaderIfChanged(writer, candidate.Project)

	renderFixPromptBlock(writer, candidate)

	inMonorepo := candidate.Project != ""

	answer, err := terminal.Confirm(a.in, a.out, "Apply fix?", "", terminal.ConfirmOptions{ShowApplyProject: inMonorepo})

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
	case terminal.AnswerApplyProject:
		if inMonorepo {
			a.autoApproveProject = candidate.Project
		}

		return engine.FixApply, nil
	case terminal.AnswerQuit:
		return engine.FixAbort, nil
	default:
		return engine.FixSkip, nil
	}
}

// resetProjectScopeIfChanged clears the "apply all remaining in this project" state when the approver moves into a different project, so each project's auto-approve choice stays scoped to that project.
func (a *TTYFixApprover) resetProjectScopeIfChanged(project string) {
	if !a.haveSeenCandidate {
		a.haveSeenCandidate = true
		a.lastPromptProject = project
		return
	}

	if project != a.lastPromptProject {
		a.autoApproveProject = ""
		a.lastPromptProject = project
	}
}

func (a *TTYFixApprover) renderProjectHeaderIfChanged(ow *terminal.OutputWriter, project string) {
	if project == "" || project == a.lastPrintedProject {
		return
	}

	a.lastPrintedProject = project

	ow.PrintNewLines(1)
	ow.Println(terminal.Bold + terminal.Cyan + "  " + project + terminal.Reset)
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
