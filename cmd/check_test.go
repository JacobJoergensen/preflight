package cmd

import (
	"slices"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/model"
)

func TestFixOfferTargetsFailingInstallableEcosystems(t *testing.T) {
	blocking := model.Message{Severity: model.SeverityError, Text: "missing dependency"}
	healthy := model.Message{Severity: model.SeveritySuccess, Text: "all good"}

	report := result.CheckReport{
		Items: []result.CheckItem{
			{ScopeID: "composer", Messages: []model.Message{blocking}},
			{ScopeID: "js", Messages: []model.Message{healthy}},
			{ScopeID: "php", Messages: []model.Message{blocking}},
			{ScopeID: "go", Messages: []model.Message{blocking}},
			{ScopeID: "composer", Messages: []model.Message{blocking}},
		},
	}

	specs := fixableFailingSpecs(report)

	got := make([]string, len(specs))
	for i, spec := range specs {
		got[i] = spec.Name
	}

	want := []string{"composer", "go"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
