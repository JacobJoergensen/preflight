package terminal

import "testing"

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
