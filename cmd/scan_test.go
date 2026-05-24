package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		format   string
		wantJSON bool
		wantErr  bool
	}{
		{"", false, false},
		{"text", false, false},
		{"json", true, false},
		{"yaml", false, true},
	}

	for _, tt := range tests {
		gotJSON, err := parseFormat(tt.format)

		if (err != nil) != tt.wantErr {
			t.Errorf("parseFormat(%q) err = %v, wantErr %v", tt.format, err, tt.wantErr)
		}

		if gotJSON != tt.wantJSON {
			t.Errorf("parseFormat(%q) = %v, want %v", tt.format, gotJSON, tt.wantJSON)
		}
	}
}

func TestReportExitCode(t *testing.T) {
	failed := func(ok bool) bool { return !ok }

	if got := reportExitCode(false, []bool{true, true}, failed); got != exitSuccess {
		t.Errorf("all healthy = %d, want %d", got, exitSuccess)
	}

	if got := reportExitCode(false, []bool{true, false}, failed); got != exitFindings {
		t.Errorf("one failed = %d, want %d", got, exitFindings)
	}

	if got := reportExitCode(true, []bool{true}, failed); got != exitFindings {
		t.Errorf("canceled = %d, want %d", got, exitFindings)
	}
}

func TestFlagOrProfile(t *testing.T) {
	newCmd := func() *cobra.Command {
		c := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return nil }}
		c.Flags().String("only", "", "")

		return c
	}

	profileVal := "from-profile"

	t.Run("profile wins when flag unchanged", func(t *testing.T) {
		if got := flagOrProfile(newCmd(), "only", "cli-default", &profileVal); got != "from-profile" {
			t.Errorf("got %q, want from-profile", got)
		}
	})

	t.Run("cli wins when flag changed", func(t *testing.T) {
		c := newCmd()
		_ = c.Flags().Set("only", "from-cli")

		if got := flagOrProfile(c, "only", "from-cli", &profileVal); got != "from-cli" {
			t.Errorf("got %q, want from-cli", got)
		}
	})

	t.Run("cli default when no profile", func(t *testing.T) {
		if got := flagOrProfile(newCmd(), "only", "cli-default", nil); got != "cli-default" {
			t.Errorf("got %q, want cli-default", got)
		}
	})
}
