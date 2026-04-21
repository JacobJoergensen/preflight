package lockdiff

import "testing"

func TestNpmParserExtractsPackagesFromLockfileV3(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"lockfileVersion": 3,
		"packages": {
			"": {"name": "root"},
			"node_modules/lodash": {"version": "4.17.21"},
			"node_modules/react": {"version": "18.2.0"}
		}
	}`)

	got, err := npmParser{}.Parse(data)

	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := map[string]string{"lodash": "4.17.21", "react": "18.2.0"}

	if len(got) != len(want) {
		t.Fatalf("got %d packages, want %d: %v", len(got), len(want), got)
	}

	for name, version := range want {
		if got[name] != version {
			t.Errorf("package %q = %q, want %q", name, got[name], version)
		}
	}
}

func TestNpmParserUsesInnermostNodeModulesSegment(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"packages": {
			"node_modules/foo/node_modules/bar": {"version": "2.0.0"}
		}
	}`)

	got, err := npmParser{}.Parse(data)

	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if got["bar"] != "2.0.0" {
		t.Errorf("nested dep bar = %q, want %q", got["bar"], "2.0.0")
	}

	if _, ok := got["foo/node_modules/bar"]; ok {
		t.Errorf("parser kept outer path as key, want innermost segment only")
	}
}

func TestNpmParserFallsBackToDependenciesWhenPackagesEmpty(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"lockfileVersion": 1,
		"dependencies": {
			"lodash": {"version": "4.17.21"}
		}
	}`)

	got, err := npmParser{}.Parse(data)

	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if got["lodash"] != "4.17.21" {
		t.Errorf("lodash = %q, want %q", got["lodash"], "4.17.21")
	}
}

func TestNpmParserReturnsErrorForInvalidJSON(t *testing.T) {
	t.Parallel()

	if _, err := (npmParser{}).Parse([]byte("not json")); err == nil {
		t.Error("Parse accepted invalid JSON, want error")
	}
}
