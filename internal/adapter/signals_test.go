package adapter

import "testing"

func TestWorkspacesNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "empty bytes",
			raw:  "",
			want: false,
		},
		{
			name: "null literal",
			raw:  "null",
			want: false,
		},
		{
			name: "empty array",
			raw:  "[]",
			want: false,
		},
		{
			name: "empty object",
			raw:  "{}",
			want: false,
		},
		{
			name: "array with single item",
			raw:  `["packages/*"]`,
			want: true,
		},
		{
			name: "array with multiple items",
			raw:  `["packages/*", "apps/*"]`,
			want: true,
		},
		{
			name: "object with packages key",
			raw:  `{"packages": ["packages/*"]}`,
			want: true,
		},
		{
			name: "whitespace only",
			raw:  "   ",
			want: false,
		},
		{
			name: "null with whitespace",
			raw:  "  null  ",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workspacesNonEmpty([]byte(tt.raw))

			if got != tt.want {
				t.Errorf("workspacesNonEmpty(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestTruncateSignalText(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		maxLen int
		want   string
	}{
		{
			name:   "shorter than max",
			text:   "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exactly max length",
			text:   "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "longer than max truncates",
			text:   "hello world",
			maxLen: 8,
			want:   "hello w…",
		},
		{
			name:   "max length 1 returns ellipsis",
			text:   "hello",
			maxLen: 1,
			want:   "…",
		},
		{
			name:   "max length 0 returns ellipsis",
			text:   "hello",
			maxLen: 0,
			want:   "…",
		},
		{
			name:   "empty text",
			text:   "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateSignalText(tt.text, tt.maxLen)

			if got != tt.want {
				t.Errorf("truncateSignalText(%q, %d) = %q, want %q",
					tt.text, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestShortSignalPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "short path unchanged",
			path: "/home/user/project",
			want: "/home/user/project",
		},
		{
			name: "exactly 96 chars unchanged",
			path: "/very/long/path/that/is/exactly/ninety/six/characters/long/here/we/go/now/abcdefghijklmnopqrstuv",
			want: "/very/long/path/that/is/exactly/ninety/six/characters/long/here/we/go/now/abcdefghijklmnopqrstuv",
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortSignalPath(tt.path)

			if got != tt.want {
				t.Errorf("shortSignalPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestShortSignalPathTruncatesLongPath(t *testing.T) {
	// Create a path longer than 96 characters
	longPath := "/home/user/very/deeply/nested/directory/structure/that/goes/on/and/on/and/on/for/a/really/long/time/project"

	got := shortSignalPath(longPath)

	if !startsWithEllipsis(got) {
		t.Errorf("expected leading ellipsis, got %q", got)
	}

	// Should keep last 95 chars of original path after ellipsis
	expectedSuffix := longPath[len(longPath)-95:]

	if got != "…"+expectedSuffix {
		t.Errorf("got %q, want ellipsis + last 95 chars", got)
	}
}

func startsWithEllipsis(text string) bool {
	return len(text) >= 3 && text[:3] == "…"
}
