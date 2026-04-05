package hooks

import (
	"strings"
	"testing"
)

func TestRemovePreflightBlock(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "removes block from middle",
			content: "#!/bin/sh\necho before\n# BEGIN PREFLIGHT\npreflight check || exit 1\n# END PREFLIGHT\necho after",
			want:    "#!/bin/sh\necho before\necho after",
		},
		{
			name:    "removes block at end",
			content: "#!/bin/sh\necho before\n# BEGIN PREFLIGHT\npreflight check || exit 1\n# END PREFLIGHT",
			want:    "#!/bin/sh\necho before",
		},
		{
			name:    "removes only block leaving shebang",
			content: "#!/bin/sh\n# BEGIN PREFLIGHT\npreflight check || exit 1\n# END PREFLIGHT",
			want:    "#!/bin/sh",
		},
		{
			name:    "handles content with no block",
			content: "#!/bin/sh\necho hello",
			want:    "#!/bin/sh\necho hello",
		},
		{
			name:    "handles empty content",
			content: "",
			want:    "",
		},
		{
			name:    "handles markers with surrounding whitespace",
			content: "  # BEGIN PREFLIGHT  \ncommand\n  # END PREFLIGHT  ",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemovePreflightBlock(tt.content)

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatBlock(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "formats custom command",
			command: "preflight check --with-env",
			want:    "# BEGIN PREFLIGHT\npreflight check --with-env || exit 1\n# END PREFLIGHT",
		},
		{
			name:    "uses default when empty",
			command: "",
			want:    "# BEGIN PREFLIGHT\npreflight check || exit 1\n# END PREFLIGHT",
		},
		{
			name:    "trims whitespace from command",
			command: "  preflight check  ",
			want:    "# BEGIN PREFLIGHT\npreflight check || exit 1\n# END PREFLIGHT",
		},
		{
			name:    "uses default for whitespace-only",
			command: "   ",
			want:    "# BEGIN PREFLIGHT\npreflight check || exit 1\n# END PREFLIGHT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBlock(tt.command)

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatePreCommitPath(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantError  bool
		errContain string
	}{
		{
			name:      "valid path",
			path:      "/project/.git/hooks/pre-commit",
			wantError: false,
		},
		{
			name:       "wrong filename",
			path:       "/project/.git/hooks/post-commit",
			wantError:  true,
			errContain: "must end with",
		},
		{
			name:       "not under hooks dir",
			path:       "/project/.git/pre-commit",
			wantError:  true,
			errContain: "must live under",
		},
		{
			name:       "not under .git dir",
			path:       "/project/hooks/pre-commit",
			wantError:  true,
			errContain: "must live under",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePreCommitPath(tt.path)

			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContain)
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
