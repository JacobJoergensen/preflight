package composer

import (
	"maps"
	"slices"
	"testing"
)

func TestParseComposerJSON(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantDep []string
		wantDev []string
	}{
		{
			name:    "filters php and ext requirements, keeps and sorts the rest",
			raw:     `{"require":{"php":"^8.2","ext-mbstring":"*","vendor/pkg":"^1.0","another/lib":"^2.0"}}`,
			wantDep: []string{"another/lib", "vendor/pkg"},
		},
		{
			name:    "filters ext from dev requirements",
			raw:     `{"require-dev":{"ext-xdebug":"*","phpunit/phpunit":"^10"}}`,
			wantDev: []string{"phpunit/phpunit"},
		},
		{
			name: "empty manifest yields no dependencies",
			raw:  `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, devDeps, err := parseComposerJSON([]byte(tt.raw))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !slices.Equal(deps, tt.wantDep) {
				t.Errorf("dependencies = %v, want %v", deps, tt.wantDep)
			}

			if !slices.Equal(devDeps, tt.wantDev) {
				t.Errorf("devDependencies = %v, want %v", devDeps, tt.wantDev)
			}
		})
	}
}

func TestParseComposerAdvisoryCounts(t *testing.T) {
	tests := []struct {
		name string
		json string
		want map[string]int
	}{
		{
			name: "counts advisories by severity across the package-keyed object",
			json: `{"advisories":{"vendor/a":[{"severity":"high"},{"severity":"high"}],"vendor/b":[{"severity":"critical"}]}}`,
			want: map[string]int{"high": 2, "critical": 1},
		},
		{
			name: "an advisory with no severity counts as medium",
			json: `{"advisories":{"vendor/a":[{"cve":"CVE-1"}]}}`,
			want: map[string]int{"medium": 1},
		},
		{
			name: "no advisories yields an empty count",
			json: `{"advisories":{}}`,
			want: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseComposerAdvisoryCounts(tt.json); !maps.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
