package python

import (
	"maps"
	"testing"
)

func TestParsePipListJSON(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   map[string]string
	}{
		{
			name:   "lowercases package names",
			output: `[{"name":"Django","version":"4.0"},{"name":"requests","version":"2.0"}]`,
			want:   map[string]string{"django": "4.0", "requests": "2.0"},
		},
		{
			name:   "skips entries with an empty name",
			output: `[{"name":"","version":"1.0"},{"name":"flask","version":"2.0"}]`,
			want:   map[string]string{"flask": "2.0"},
		},
		{
			name:   "invalid json yields an empty map",
			output: `not json`,
			want:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePipListJSON(tt.output); !maps.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
