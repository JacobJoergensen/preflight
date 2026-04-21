package lockdiff

import "testing"

func TestGoSumParserStripsGoModSuffix(t *testing.T) {
	t.Parallel()

	data := []byte("github.com/spf13/cobra v1.10.1/go.mod h1:abc\n")

	got, err := goSumParser{}.Parse(data)

	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if got["github.com/spf13/cobra"] != "v1.10.1" {
		t.Errorf("version = %q, want %q (go.mod suffix should be stripped)", got["github.com/spf13/cobra"], "v1.10.1")
	}
}

func TestGoSumParserKeepsHighestVersionWhenModuleAppearsMultipleTimes(t *testing.T) {
	t.Parallel()

	data := []byte(`github.com/foo/bar v1.0.0 h1:abc
github.com/foo/bar v1.0.0/go.mod h1:def
github.com/foo/bar v1.2.0 h1:ghi
github.com/foo/bar v1.2.0/go.mod h1:jkl
`)

	got, err := goSumParser{}.Parse(data)

	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if got["github.com/foo/bar"] != "v1.2.0" {
		t.Errorf("version = %q, want %q (should keep highest)", got["github.com/foo/bar"], "v1.2.0")
	}
}

func TestGoSumParserIgnoresBlankAndShortLines(t *testing.T) {
	t.Parallel()

	data := []byte(`
github.com/foo/bar v1.0.0 h1:abc

malformed
github.com/baz/qux v2.0.0 h1:xyz
`)

	got, err := goSumParser{}.Parse(data)

	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d modules, want 2: %v", len(got), got)
	}

	if got["github.com/foo/bar"] != "v1.0.0" || got["github.com/baz/qux"] != "v2.0.0" {
		t.Errorf("unexpected parse result: %v", got)
	}
}
