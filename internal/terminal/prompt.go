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

const maxPromptAttempts = 3

func Confirm(in io.Reader, out io.Writer, question, hint string, opts ConfirmOptions) (Answer, error) {
	return prompt(in, out,
		func() error { return writePrompt(out, question, hint, opts) },
		func(line string) (Answer, bool) { return parseAnswer(line, opts) },
		AnswerYes,
		promptInvalidMessage(opts),
	)
}

func prompt[T any](in io.Reader, out io.Writer, render func() error, parse func(string) (T, bool), onEOF T, invalidMessage string) (T, error) {
	var zero T

	if in == nil || out == nil {
		return zero, errors.New("terminal: prompt requires non-nil reader and writer")
	}

	reader := bufio.NewReader(in)

	for range maxPromptAttempts {
		if err := render(); err != nil {
			return zero, err
		}

		line, err := reader.ReadString('\n')

		if err != nil && (!errors.Is(err, io.EOF) || line == "") {
			if errors.Is(err, io.EOF) {
				return onEOF, nil
			}

			return zero, fmt.Errorf("read prompt response: %w", err)
		}

		if value, ok := parse(line); ok {
			return value, nil
		}

		if _, err := fmt.Fprintln(out, Dim+"  "+invalidMessage+Reset); err != nil {
			return zero, err
		}
	}

	return zero, errors.New("terminal: too many invalid responses")
}

func Ask(in io.Reader, out io.Writer, question string) (bool, error) {
	render := func() error {
		_, err := fmt.Fprint(out, "  "+Cyan+"?"+Reset+" "+Bold+question+Reset+" "+Dim+"[y/N]"+Reset+" ")
		return err
	}

	parse := func(line string) (bool, bool) {
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return true, true
		case "", "n", "no":
			return false, true
		default:
			return false, false
		}
	}

	return prompt(in, out, render, parse, false, "Please answer y or n.")
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

func IsInteractive() bool {
	return IsInteractiveTTY(os.Stdin) && IsInteractiveTTY(os.Stdout)
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
