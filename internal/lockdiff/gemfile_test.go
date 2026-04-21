package lockdiff

import "testing"

func TestGemfileParserExtractsGemsFromGemSection(t *testing.T) {
	t.Parallel()

	data := []byte(`GEM
  remote: https://rubygems.org/
  specs:
    rails (7.1.0)
      actionpack (= 7.1.0)
    actionpack (7.1.0)
      rack (~> 2.2)

PLATFORMS
  ruby

DEPENDENCIES
  rails

BUNDLED WITH
   2.4.0
`)

	got, err := gemfileParser{}.Parse(data)

	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	want := map[string]string{
		"rails":      "7.1.0",
		"actionpack": "7.1.0",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d gems, want %d: %v", len(got), len(want), got)
	}

	for name, version := range want {
		if got[name] != version {
			t.Errorf("gem %q = %q, want %q", name, got[name], version)
		}
	}
}

func TestGemfileParserIgnoresPathAndGitSections(t *testing.T) {
	t.Parallel()

	data := []byte(`PATH
  remote: vendor/my-gem
  specs:
    my-gem (0.1.0)

GIT
  remote: https://github.com/example/other.git
  specs:
    other-gem (2.0.0)

GEM
  remote: https://rubygems.org/
  specs:
    rails (7.1.0)
`)

	got, err := gemfileParser{}.Parse(data)

	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if _, ok := got["my-gem"]; ok {
		t.Errorf("parser included gem from PATH section, want GEM-only")
	}

	if _, ok := got["other-gem"]; ok {
		t.Errorf("parser included gem from GIT section, want GEM-only")
	}

	if got["rails"] != "7.1.0" {
		t.Errorf("rails = %q, want %q", got["rails"], "7.1.0")
	}
}
