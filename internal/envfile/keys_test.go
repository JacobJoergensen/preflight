package envfile

import (
	"slices"
	"testing"
)

func TestParseKeys(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "simple key value",
			content: "FOO=bar",
			want:    []string{"FOO"},
		},
		{
			name:    "multiple keys",
			content: "FOO=bar\nBAZ=qux",
			want:    []string{"FOO", "BAZ"},
		},
		{
			name:    "skips empty lines",
			content: "FOO=bar\n\nBAZ=qux",
			want:    []string{"FOO", "BAZ"},
		},
		{
			name:    "skips comments",
			content: "# comment\nFOO=bar\n# another\nBAZ=qux",
			want:    []string{"FOO", "BAZ"},
		},
		{
			name:    "handles export prefix",
			content: "export FOO=bar",
			want:    []string{"FOO"},
		},
		{
			name:    "handles EXPORT uppercase",
			content: "EXPORT FOO=bar",
			want:    []string{"FOO"},
		},
		{
			name:    "deduplicates preserving first",
			content: "FOO=first\nBAR=middle\nFOO=second",
			want:    []string{"FOO", "BAR"},
		},
		{
			name:    "trims whitespace around key",
			content: "  FOO  =bar",
			want:    []string{"FOO"},
		},
		{
			name:    "strips quotes from key",
			content: `"FOO"=bar`,
			want:    []string{"FOO"},
		},
		{
			name:    "skips lines without equals",
			content: "FOO=bar\nINVALID\nBAZ=qux",
			want:    []string{"FOO", "BAZ"},
		},
		{
			name:    "skips invalid key names",
			content: "FOO=bar\n123INVALID=x\nBAZ=qux",
			want:    []string{"FOO", "BAZ"},
		},
		{
			name:    "allows underscores and dots in keys",
			content: "FOO_BAR=x\nBAZ.QUX=y",
			want:    []string{"FOO_BAR", "BAZ.QUX"},
		},
		{
			name:    "allows underscore as first char",
			content: "_PRIVATE=secret",
			want:    []string{"_PRIVATE"},
		},
		{
			name:    "empty content returns nil",
			content: "",
			want:    nil,
		},
		{
			name:    "only comments returns nil",
			content: "# just a comment\n# another",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseKeys([]byte(tt.content))

			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseKeysWithBOM(t *testing.T) {
	content := []byte("\xEF\xBB\xBFFOO=bar")

	got := ParseKeys(content)
	want := []string{"FOO"}

	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIsValidEnvKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"FOO", true},
		{"foo", true},
		{"FOO_BAR", true},
		{"FOO.BAR", true},
		{"_PRIVATE", true},
		{"A1", true},
		{"FOO123", true},

		{"", false},
		{"123FOO", false},
		{".FOO", false},
		{"FOO-BAR", false},
		{"FOO BAR", false},
		{"FOO=BAR", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := isValidEnvKey(tt.key)

			if got != tt.want {
				t.Errorf("isValidEnvKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
