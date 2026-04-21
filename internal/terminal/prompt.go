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
	default:
		return "unknown"
	}
}

func Confirm(in io.Reader, out io.Writer, question, hint string) (Answer, error) {
	if in == nil || out == nil {
		return 0, errors.New("terminal: Confirm requires non-nil reader and writer")
	}

	reader := bufio.NewReader(in)

	for range maxPromptAttempts {
		if err := writePrompt(out, question, hint); err != nil {
			return 0, err
		}

		line, err := reader.ReadString('\n')

		if err != nil && (!errors.Is(err, io.EOF) || line == "") {
			if errors.Is(err, io.EOF) {
				return AnswerYes, nil
			}

			return 0, fmt.Errorf("read prompt response: %w", err)
		}

		answer, ok := parseAnswer(line)

		if ok {
			return answer, nil
		}

		if _, err := fmt.Fprintln(out, Dim+"  Please answer y, n, a, or q."+Reset); err != nil {
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

func writePrompt(out io.Writer, question, hint string) error {
	line := "  " + Cyan + "?" + Reset + " " + Bold + question + Reset

	if hint != "" {
		line += " " + Dim + hint + Reset
	}

	line += " " + Dim + "[Y/n/a/q]" + Reset + " "

	_, err := fmt.Fprint(out, line)
	return err
}

func parseAnswer(input string) (Answer, bool) {
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
	default:
		return 0, false
	}
}
