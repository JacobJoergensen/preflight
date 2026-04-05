package engine

import (
	"errors"
	"fmt"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/adapter"
	"github.com/JacobJoergensen/preflight/internal/manifest"
)

type Mode string

const (
	ModeCheck Mode = "check"
	ModeList  Mode = "list"
	ModeFix   Mode = "fix"
	ModeAudit Mode = "audit"
)

type Selection struct {
	Adapters      []adapter.Adapter
	AdapterIDs    []string
	FixSelectors  []string
	RequestedMode Mode
}

type SelectInput struct {
	Scopes    []string
	Selectors []string
	Mode      Mode
}

func Select(input SelectInput) (Selection, error) {
	scopes := normalizeNames(input.Scopes)
	selectors := normalizeNames(input.Selectors)

	if len(scopes) > 0 && len(selectors) > 0 {
		return Selection{}, errors.New("cannot use both scopes and selectors")
	}

	if len(scopes) > 0 {
		return selectByScopes(scopes, input.Mode)
	}

	normalized := selectors

	if len(normalized) == 0 {
		adapters, err := adapter.Select()

		if err != nil {
			return Selection{}, err
		}

		return Selection{
			Adapters:      adapters,
			AdapterIDs:    adapter.Names(adapters),
			FixSelectors:  nil,
			RequestedMode: input.Mode,
		}, nil
	}

	switch input.Mode {
	case ModeCheck, ModeList, ModeAudit:
		ids := make([]string, 0, len(normalized))

		for _, name := range normalized {
			ids = append(ids, manifest.ResolvePackageType(name))
		}

		adapters, err := adapter.Select(ids...)

		if err != nil {
			return Selection{}, err
		}

		return Selection{
			Adapters:      adapters,
			AdapterIDs:    adapter.Names(adapters),
			FixSelectors:  nil,
			RequestedMode: input.Mode,
		}, nil
	case ModeFix:
		return selectForFix(normalized)
	default:
		return Selection{}, fmt.Errorf("unknown selection mode: %s", input.Mode)
	}
}

func selectByScopes(scopes []string, mode Mode) (Selection, error) {
	for _, s := range scopes {
		if !adapter.IsRegistered(s) {
			return Selection{}, fmt.Errorf("unknown scope: %s", s)
		}
	}

	adapters, err := adapter.Select(scopes...)

	if err != nil {
		return Selection{}, err
	}

	return Selection{
		Adapters:      adapters,
		AdapterIDs:    adapter.Names(adapters),
		FixSelectors:  scopes,
		RequestedMode: mode,
	}, nil
}

func selectForFix(normalized []string) (Selection, error) {
	var errs []string

	fixSelectors := make([]string, 0, len(normalized))
	adapterSet := make(map[string]struct{})
	selectedIDs := make([]string, 0, len(normalized))

	for _, name := range normalized {
		if manifest.IsPackageType(name) {
			fixSelectors = append(fixSelectors, name)

			if _, ok := adapterSet[name]; !ok {
				adapterSet[name] = struct{}{}
				selectedIDs = append(selectedIDs, name)
			}

			continue
		}

		packageType, ok := manifest.GetPackageType(name)

		if !ok {
			errs = append(errs, "unknown selector: "+name)
			continue
		}

		fixSelectors = append(fixSelectors, name)

		if _, ok := adapterSet[packageType]; !ok {
			adapterSet[packageType] = struct{}{}
			selectedIDs = append(selectedIDs, packageType)
		}
	}

	if len(errs) > 0 {
		return Selection{}, fmt.Errorf("selection errors: %s", strings.Join(errs, "; "))
	}

	adapters, err := adapter.Select(selectedIDs...)

	if err != nil {
		return Selection{}, err
	}

	return Selection{
		Adapters:      adapters,
		AdapterIDs:    adapter.Names(adapters),
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
