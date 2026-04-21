package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	gofs "io/fs"
	"os"
	"strings"
)

type Answer int

const (
	AnswerYes Answer = iota
	AnswerNo
	AnswerAll
	AnswerQuit
	AnswerApplyProject
)

const maxPromptAttempts = 3

func (a Answer) String() string {
	switch a {
	case AnswerYes:
		return "yes"
	case AnswerNo:
		return "no"
	case AnswerAll:
		return "all"
	case AnswerQuit:
		return "quit"
	case AnswerApplyProject:
		return "project"
	default:
		return "unknown"
	}
}

type ConfirmOptions struct {
	ShowApplyProject bool
}

func Confirm(in io.Reader, out io.Writer, question, hint string, opts ConfirmOptions) (Answer, error) {
	if in == nil || out == nil {
		return 0, errors.New("terminal: Confirm requires non-nil reader and writer")
	}

	reader := bufio.NewReader(in)

	for range maxPromptAttempts {
		if err := writePrompt(out, question, hint, opts); err != nil {
			return 0, err
		}

		line, err := reader.ReadString('\n')

		if err != nil && (!errors.Is(err, io.EOF) || line == "") {
			if errors.Is(err, io.EOF) {
				return AnswerYes, nil
			}

			return 0, fmt.Errorf("read prompt response: %w", err)
		}

		answer, ok := parseAnswer(line, opts)

		if ok {
			return answer, nil
		}

		if _, err := fmt.Fprintln(out, Dim+"  "+promptInvalidMessage(opts)+Reset); err != nil {
			return 0, err
		}
	}

	return 0, errors.New("terminal: too many invalid responses")
}

func IsInteractiveTTY(f *os.File) bool {
	if f == nil {
		return false
	}

	info, err := f.Stat()

	if err != nil {
		return false
	}

	return info.Mode()&gofs.ModeCharDevice != 0
}

func writePrompt(out io.Writer, question, hint string, opts ConfirmOptions) error {
	line := "  " + Cyan + "?" + Reset + " " + Bold + question + Reset

	if hint != "" {
		line += " " + Dim + hint + Reset
	}

	line += " " + Dim + promptChoicesLabel(opts) + Reset + " "

	_, err := fmt.Fprint(out, line)
	return err
}

func promptChoicesLabel(opts ConfirmOptions) string {
	if opts.ShowApplyProject {
		return "[Y/n/a/p/q]"
	}

	return "[Y/n/a/q]"
}

func promptInvalidMessage(opts ConfirmOptions) string {
	if opts.ShowApplyProject {
		return "Please answer y, n, a, p, or q."
	}

	return "Please answer y, n, a, or q."
}

func parseAnswer(input string, opts ConfirmOptions) (Answer, bool) {
	trimmed := strings.ToLower(strings.TrimSpace(input))

	switch trimmed {
	case "", "y", "yes":
		return AnswerYes, true
	case "n", "no":
		return AnswerNo, true
	case "a", "all":
		return AnswerAll, true
	case "q", "quit", "abort":
		return AnswerQuit, true
	case "p", "project":
		if opts.ShowApplyProject {
			return AnswerApplyProject, true
		}

		return 0, false
	default:
		return 0, false
	}
}
