package ruby

import (
	"slices"
	"testing"
)

func TestParseGemfileGemNames(t *testing.T) {
	tests := []struct {
		name    string
		gemfile string
		want    []string
	}{
		{
			name:    "collects gem names and ignores other directives",
			gemfile: "source \"https://rubygems.org\"\ngem \"rails\"\ngem 'rspec'\n",
			want:    []string{"rails", "rspec"},
		},
		{
			name:    "deduplicates case-insensitively keeping the first spelling",
			gemfile: "gem \"rails\"\ngem \"Rails\"\n",
			want:    []string{"rails"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseGemfileGemNames(tt.gemfile); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
