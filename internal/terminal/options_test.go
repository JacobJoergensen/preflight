package terminal

import "testing"

func TestColorEnabled(t *testing.T) {
	tests := []struct {
		name       string
		disabled   bool
		forceColor string
		noColor    string
		want       bool
	}{
		{name: "disable flag wins over force", disabled: true, forceColor: "1", want: false},
		{name: "force color overrides no color", forceColor: "1", noColor: "1", want: true},
		{name: "no color disables", noColor: "1", want: false},
		{name: "non-tty disables when unset", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FORCE_COLOR", tt.forceColor)
			t.Setenv("NO_COLOR", tt.noColor)

			if got := colorEnabled(tt.disabled, nil); got != tt.want {
				t.Errorf("colorEnabled(%v) = %v, want %v", tt.disabled, got, tt.want)
			}
		})
	}
}
