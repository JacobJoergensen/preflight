package ruby

import (
	"slices"
	"testing"
)

func TestParseLicenseFinderCSV(t *testing.T) {
	output := "name,version,licenses\n" +
		"rails,7.0.0,MIT\n" +
		"nokogiri,1.13.0,\"MIT, Apache-2.0\"\n"

	packages := parseLicenseFinderCSV(output)

	if len(packages) != 2 {
		t.Fatalf("got %d packages, want 2 (header skipped)", len(packages))
	}

	// Sorted by name; the quoted multi-license field stays intact.
	if packages[0].Name != "nokogiri" || packages[0].Version != "1.13.0" || packages[0].License != "MIT, Apache-2.0" {
		t.Errorf("packages[0] = %+v", packages[0])
	}

	if packages[1].Name != "rails" || packages[1].License != "MIT" {
		t.Errorf("packages[1] = %+v", packages[1])
	}
}

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
