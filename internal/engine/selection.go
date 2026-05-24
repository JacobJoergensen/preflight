package engine

import (
	"fmt"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
)

type Mode string

const (
	ModeCheck Mode = "check"
	ModeFix   Mode = "fix"
	ModeAudit Mode = "audit"
)

type Selection struct {
	Specs         []*ecosystem.Spec
	SpecIDs       []string
	FixSelectors  []string
	RequestedMode Mode
}

type SelectInput struct {
	Only []string
	Mode Mode
}

func Select(input SelectInput) (Selection, error) {
	only := normalizeNames(input.Only)

	if len(only) == 0 {
		specs, err := ecosystem.Select()
		if err != nil {
			return Selection{}, err
		}

		return Selection{
			Specs:         specs,
			SpecIDs:       ecosystem.Names(specs),
			FixSelectors:  nil,
			RequestedMode: input.Mode,
		}, nil
	}

	switch input.Mode {
	case ModeCheck, ModeAudit:
		ids := make([]string, 0, len(only))

		for _, name := range only {
			ids = append(ids, ecosystem.ResolveScope(name))
		}

		specs, err := ecosystem.Select(ids...)
		if err != nil {
			return Selection{}, err
		}

		return Selection{
			Specs:         specs,
			SpecIDs:       ecosystem.Names(specs),
			FixSelectors:  nil,
			RequestedMode: input.Mode,
		}, nil
	case ModeFix:
		return selectForFix(only)
	default:
		return Selection{}, fmt.Errorf("unknown selection mode: %s", input.Mode)
	}
}

func selectForFix(normalized []string) (Selection, error) {
	var errs []string

	fixSelectors := make([]string, 0, len(normalized))
	specSet := make(map[string]struct{})
	selectedIDs := make([]string, 0, len(normalized))

	for _, name := range normalized {
		if ecosystem.IsScope(name) {
			fixSelectors = append(fixSelectors, name)

			if _, ok := specSet[name]; !ok {
				specSet[name] = struct{}{}
				selectedIDs = append(selectedIDs, name)
			}

			continue
		}

		scope, ok := ecosystem.ScopeForManager(name)

		if !ok {
			errs = append(errs, "unknown selector: "+name)
			continue
		}

		fixSelectors = append(fixSelectors, name)

		if _, ok := specSet[scope]; !ok {
			specSet[scope] = struct{}{}
			selectedIDs = append(selectedIDs, scope)
		}
	}

	if len(errs) > 0 {
		return Selection{}, fmt.Errorf("selection errors: %s", strings.Join(errs, "; "))
	}

	specs, err := ecosystem.Select(selectedIDs...)
	if err != nil {
		return Selection{}, err
	}

	return Selection{
		Specs:         specs,
		SpecIDs:       ecosystem.Names(specs),
		FixSelectors:  dedupePreserveOrder(fixSelectors),
		RequestedMode: ModeFix,
	}, nil
}

func normalizeNames(names []string) []string {
	result := make([]string, 0, len(names))

	for _, name := range names {
		trimmed := strings.ToLower(strings.TrimSpace(name))

		if trimmed == "" {
			continue
		}

		result = append(result, trimmed)
	}

	return result
}

func dedupePreserveOrder(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))

	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}
		result = append(result, name)
	}

	return result
}
