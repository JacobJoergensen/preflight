package js

import "testing"

func TestShouldUseNodeEnginesSemverRange(t *testing.T) {
	tests := []struct {
		name    string
		engines string
		want    bool
	}{
		{name: "plain version is an exact pin", engines: "18.0.0", want: false},
		{name: "caret is a range", engines: "^18.0.0", want: true},
		{name: "comparator is a range", engines: ">=18", want: true},
		{name: "or is a range", engines: "18 || 20", want: true},
		{name: "hyphen is a range", engines: "18 - 20", want: true},
		{name: "x-range is a range", engines: "18.x", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseNodeEnginesSemverRange(tt.engines); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
