package adapter

import "testing"

func TestNodeEngineSatisfiedByRuntime(t *testing.T) {
	tests := []struct {
		name      string
		installed string
		engines   string
		want      bool
	}{
		{
			name:      "empty engines always satisfied",
			installed: "20.0.0",
			engines:   "",
			want:      true,
		},
		{
			name:      "whitespace engines satisfied",
			installed: "20.0.0",
			engines:   "   ",
			want:      true,
		},
		{
			name:      "bare version minimum satisfied",
			installed: "20.5.0",
			engines:   "20",
			want:      true,
		},
		{
			name:      "bare version minimum not satisfied",
			installed: "18.0.0",
			engines:   "20",
			want:      false,
		},
		{
			name:      "exact semver satisfied",
			installed: "20.0.0",
			engines:   "20.0.0",
			want:      true,
		},
		{
			name:      "strips v prefix from installed",
			installed: "v20.5.0",
			engines:   "20",
			want:      true,
		},
		{
			name:      "caret range satisfied",
			installed: "20.5.0",
			engines:   "^20.0.0",
			want:      true,
		},
		{
			name:      "caret range not satisfied",
			installed: "21.0.0",
			engines:   "^20.0.0",
			want:      false,
		},
		{
			name:      "tilde range satisfied",
			installed: "20.0.5",
			engines:   "~20.0.0",
			want:      true,
		},
		{
			name:      "greater equal satisfied",
			installed: "20.0.0",
			engines:   ">=18.0.0",
			want:      true,
		},
		{
			name:      "or range satisfied first",
			installed: "18.0.0",
			engines:   "18.x || 20.x",
			want:      true,
		},
		{
			name:      "or range satisfied second",
			installed: "20.0.0",
			engines:   "18.x || 20.x",
			want:      true,
		},
		{
			name:      "x wildcard satisfied",
			installed: "20.5.0",
			engines:   "20.x",
			want:      true,
		},
		{
			name:      "hyphen range satisfied",
			installed: "19.0.0",
			engines:   "18.0.0 - 20.0.0",
			want:      true,
		},
		{
			name:      "star wildcard satisfied",
			installed: "20.0.0",
			engines:   "*",
			want:      true,
		},
		{
			name:      "less than not satisfied",
			installed: "20.0.0",
			engines:   "<18.0.0",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeEngineSatisfiedByRuntime(tt.installed, tt.engines)

			if got != tt.want {
				t.Errorf("nodeEngineSatisfiedByRuntime(%q, %q) = %v, want %v",
					tt.installed, tt.engines, got, tt.want)
			}
		})
	}
}

func TestShouldUseNodeEnginesSemverRange(t *testing.T) {
	tests := []struct {
		engines string
		want    bool
	}{
		{"20", false},
		{"20.0.0", false},
		{"^20.0.0", true},
		{"~20.0.0", true},
		{">=18.0.0", true},
		{"<=20.0.0", true},
		{">18.0.0", true},
		{"<20.0.0", true},
		{"18.x || 20.x", true},
		{"18.0.0 - 20.0.0", true},
		{"*", true},
		{"20.x", true},
		{"20.X", true},
	}

	for _, tt := range tests {
		t.Run(tt.engines, func(t *testing.T) {
			got := shouldUseNodeEnginesSemverRange(tt.engines)

			if got != tt.want {
				t.Errorf("shouldUseNodeEnginesSemverRange(%q) = %v, want %v",
					tt.engines, got, tt.want)
			}
		})
	}
}
