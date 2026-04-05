package adapter

import "testing"

func TestTrimFirstLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single line",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "multiple lines returns first",
			input: "first\nsecond\nthird",
			want:  "first",
		},
		{
			name:  "trims surrounding whitespace",
			input: "  hello  ",
			want:  "hello",
		},
		{
			name:  "trims whitespace from first line",
			input: "  first  \nsecond",
			want:  "first",
		},
		{
			name:  "trims leading newline then returns first line",
			input: "\nactual first\nsecond",
			want:  "actual first",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace",
			input: "   ",
			want:  "",
		},
		{
			name:  "windows line endings",
			input: "first\r\nsecond",
			want:  "first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimFirstLine(tt.input)

			if got != tt.want {
				t.Errorf("trimFirstLine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLookupInstalledLower(t *testing.T) {
	installed := map[string]string{
		"php":      "8.2.0",
		"node":     "20.0.0",
		"composer": "2.5.0",
	}

	tests := []struct {
		name   string
		key    string
		want   string
		wantOK bool
	}{
		{
			name:   "exact match lowercase",
			key:    "php",
			want:   "8.2.0",
			wantOK: true,
		},
		{
			name:   "uppercase key matches",
			key:    "PHP",
			want:   "8.2.0",
			wantOK: true,
		},
		{
			name:   "mixed case matches",
			key:    "NoDE",
			want:   "20.0.0",
			wantOK: true,
		},
		{
			name:   "not found",
			key:    "ruby",
			want:   "",
			wantOK: false,
		},
		{
			name:   "empty key",
			key:    "",
			want:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := lookupInstalledLower(installed, tt.key)

			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}

			if got != tt.want {
				t.Errorf("version = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLookupInstalledLowerEmptyMap(t *testing.T) {
	got, ok := lookupInstalledLower(map[string]string{}, "php")

	if ok {
		t.Error("expected false for empty map")
	}

	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
