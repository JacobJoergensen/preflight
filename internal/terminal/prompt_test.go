package terminal

import (
	"bytes"
	"strings"
	"testing"
)

func TestAsk(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"y is yes", "y\n", true},
		{"yes word", "yes\n", true},
		{"n is no", "n\n", false},
		{"empty defaults to no", "\n", false},
		{"eof defaults to no", "", false},
		{"invalid then yes", "maybe\ny\n", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer

			got, err := Ask(strings.NewReader(tt.input), &out, "Proceed?")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAnswer(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		opts   ConfirmOptions
		want   Answer
		wantOK bool
	}{
		{"empty defaults to yes", "", ConfirmOptions{}, AnswerYes, true},
		{"y", "y", ConfirmOptions{}, AnswerYes, true},
		{"yes uppercased and trimmed", "  YES ", ConfirmOptions{}, AnswerYes, true},
		{"n", "n", ConfirmOptions{}, AnswerNo, true},
		{"no", "no", ConfirmOptions{}, AnswerNo, true},
		{"a", "a", ConfirmOptions{}, AnswerAll, true},
		{"all", "all", ConfirmOptions{}, AnswerAll, true},
		{"q", "q", ConfirmOptions{}, AnswerQuit, true},
		{"abort", "abort", ConfirmOptions{}, AnswerQuit, true},
		{"project when enabled", "p", ConfirmOptions{ShowApplyProject: true}, AnswerApplyProject, true},
		{"project word when enabled", "project", ConfirmOptions{ShowApplyProject: true}, AnswerApplyProject, true},
		{"project rejected when disabled", "p", ConfirmOptions{}, 0, false},
		{"unknown rejected", "maybe", ConfirmOptions{}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseAnswer(tt.input, tt.opts)

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if ok && got != tt.want {
				t.Errorf("answer = %v, want %v", got, tt.want)
			}
		})
	}
}
