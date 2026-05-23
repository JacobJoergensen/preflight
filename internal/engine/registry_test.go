package engine

import "testing"

func TestRegisteredSpecsAreWellFormed(t *testing.T) {
	names := make(map[string]bool)
	priorities := make(map[int]string)

	for _, spec := range registeredSpecs {
		if spec.Name == "" {
			t.Errorf("spec with DisplayName %q has an empty Name", spec.DisplayName)
			continue
		}

		if names[spec.Name] {
			t.Errorf("duplicate spec Name %q", spec.Name)
		}

		names[spec.Name] = true

		if existing, ok := priorities[spec.Priority]; ok {
			t.Errorf("spec %q shares Priority %d with %q", spec.Name, spec.Priority, existing)
		}

		priorities[spec.Priority] = spec.Name

		if spec.Check == nil {
			t.Errorf("spec %q has a nil Check hook", spec.Name)
		}

		if len(spec.Detect) == 0 && len(spec.Managers) == 0 && !spec.AlwaysPresent {
			t.Errorf("spec %q can never resolve: no Detect markers, no Managers, and not AlwaysPresent", spec.Name)
		}

		commands := make(map[string]bool)

		for i, manager := range spec.Managers {
			if manager.Command == "" {
				t.Errorf("spec %q manager %d has an empty Command", spec.Name, i)
			}

			commands[manager.Command] = true

			if manager.Outdated != nil && manager.Outdated.Parse == nil {
				t.Errorf("spec %q manager %q declares an Outdated probe with a nil Parse", spec.Name, manager.Command)
			}

			if manager.Audit != nil && manager.Audit.Parse == nil {
				t.Errorf("spec %q manager %q declares an Audit probe with a nil Parse", spec.Name, manager.Command)
			}
		}

		for i, marker := range spec.Detect {
			if marker.Manager != "" && !commands[marker.Manager] {
				t.Errorf("spec %q Detect[%d] names manager %q absent from Managers", spec.Name, i, marker.Manager)
			}
		}
	}
}
