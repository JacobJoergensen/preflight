package ecosystem

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

var (
	specs   []*Spec
	allowed = map[string]struct{}{}
)

func SetSpecs(list []*Spec) {
	specs = list
	allowed = make(map[string]struct{})

	for _, spec := range list {
		for _, command := range spec.Commands() {
			allowed[command] = struct{}{}
		}
	}
}

func Gate(name string) bool {
	_, ok := allowed[name]
	return ok
}

func (s *Spec) Commands() []string {
	var commands []string

	for _, manager := range s.Managers {
		commands = append(commands, manager.Command)

		if manager.Audit != nil && manager.Audit.Tool != "" {
			commands = append(commands, manager.Audit.Tool)
		}

		if manager.Outdated != nil && manager.Outdated.Tool != "" {
			commands = append(commands, manager.Outdated.Tool)
		}
	}

	commands = append(commands, s.RuntimeCommands...)

	return commands
}

func Select(names ...string) ([]*Spec, error) {
	if len(names) == 0 {
		return sortedByPriority(specs), nil
	}

	selected := make([]*Spec, 0, len(names))
	seen := make(map[string]struct{}, len(names))

	var errs []string

	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))

		if name == "" {
			continue
		}

		if _, ok := seen[name]; ok {
			continue
		}

		spec, ok := Lookup(name)

		if !ok {
			errs = append(errs, "unknown ecosystem: "+name)
			continue
		}

		seen[name] = struct{}{}
		selected = append(selected, spec)
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("selection errors: %s", strings.Join(errs, "; "))
	}

	return sortedByPriority(selected), nil
}

func Lookup(name string) (*Spec, bool) {
	name = strings.ToLower(strings.TrimSpace(name))

	for _, spec := range specs {
		if spec.Name == name {
			return spec, true
		}
	}

	return nil, false
}

func Names(list []*Spec) []string {
	names := make([]string, 0, len(list))

	for _, spec := range list {
		names = append(names, spec.Name)
	}

	return names
}

func IsScope(name string) bool {
	spec, ok := Lookup(name)
	return ok && len(spec.Managers) > 0
}

func ScopeForManager(command string) (string, bool) {
	command = strings.ToLower(strings.TrimSpace(command))

	for _, spec := range specs {
		for _, manager := range spec.Managers {
			if manager.Command == command {
				return spec.Name, true
			}
		}
	}

	return "", false
}

func ResolveScope(name string) string {
	if scope, ok := ScopeForManager(name); ok {
		return scope
	}

	return name
}

func sortedByPriority(list []*Spec) []*Spec {
	sorted := slices.Clone(list)

	slices.SortStableFunc(sorted, func(first, second *Spec) int {
		return cmp.Compare(first.Priority, second.Priority)
	})

	return sorted
}
