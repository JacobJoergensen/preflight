package render

import (
	"slices"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
)

func TestHealthStatusFromItem(t *testing.T) {
	tests := []struct {
		name string
		item result.CheckItem
		want HealthStatus
	}{
		{
			name: "errors returns fail",
			item: result.CheckItem{Errors: []model.Message{{Text: "error"}}},
			want: HealthFail,
		},
		{
			name: "warnings only returns warn",
			item: result.CheckItem{Warnings: []model.Message{{Text: "warning"}}},
			want: HealthWarn,
		},
		{
			name: "no errors or warnings returns ok",
			item: result.CheckItem{},
			want: HealthOK,
		},
		{
			name: "errors take precedence over warnings",
			item: result.CheckItem{
				Errors:   []model.Message{{Text: "error"}},
				Warnings: []model.Message{{Text: "warning"}},
			},
			want: HealthFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := healthStatusFromItem(tt.item)

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsProjectSignalLine(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"package.json found:", true},
		{"composer.json found:", true},
		{"go.mod found", true},
		{"package.json found", true},
		{"random text", false},
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := isProjectSignalLine(tt.text)

			if got != tt.want {
				t.Errorf("isProjectSignalLine(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestExtractRunCommands(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "extracts single command",
			text: "Run `npm install` to fix",
			want: []string{"npm install"},
		},
		{
			name: "extracts multiple commands",
			text: "Run `npm install` or run `yarn install`",
			want: []string{"npm install", "yarn install"},
		},
		{
			name: "handles lowercase run",
			text: "run `composer install` first",
			want: []string{"composer install"},
		},
		{
			name: "no commands returns nil",
			text: "no commands here",
			want: nil,
		},
		{
			name: "ignores unclosed backtick",
			text: "Run `npm install without closing",
			want: nil,
		},
		{
			name: "ignores empty command",
			text: "Run `` is empty",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRunCommands(tt.text)

			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluralS(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{0, "s"},
		{1, ""},
		{2, "s"},
		{10, "s"},
	}

	for _, tt := range tests {
		got := pluralS(tt.count)

		if got != tt.want {
			t.Errorf("pluralS(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestDependencySummaryPhrase(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{1, "1 missing or invalid dependency"},
		{2, "2 missing or invalid dependencies"},
		{5, "5 missing or invalid dependencies"},
	}

	for _, tt := range tests {
		got := dependencySummaryPhrase(tt.count)

		if got != tt.want {
			t.Errorf("dependencySummaryPhrase(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}
