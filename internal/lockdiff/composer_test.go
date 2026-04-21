package lockdiff

import "testing"

func TestComposerParserMergesRuntimeAndDevPackages(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"packages": [
			{"name": "symfony/console", "version": "6.4.0"},
			{"name": "psr/log", "version": "3.0.0"}
		],
		"packages-dev": [
			{"name": "phpunit/phpunit", "version": "10.5.0"}
		]
	}`)

	got, err := composerParser{}.Parse(data)

	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := map[string]string{
		"symfony/console": "6.4.0",
		"psr/log":         "3.0.0",
		"phpunit/phpunit": "10.5.0",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d packages, want %d: %v", len(got), len(want), got)
	}

	for name, version := range want {
		if got[name] != version {
			t.Errorf("package %q = %q, want %q", name, got[name], version)
		}
	}
}

func TestComposerParserReturnsErrorForInvalidJSON(t *testing.T) {
	t.Parallel()

	if _, err := (composerParser{}).Parse([]byte("not json")); err == nil {
		t.Error("Parse accepted invalid JSON, want error")
	}
}
