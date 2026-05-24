package memfs

import (
	"path/filepath"
	"testing"
)

func TestReadDir(t *testing.T) {
	fs := New(map[string][]byte{
		filepath.Join("root", "a.txt"):        []byte("a"),
		filepath.Join("root", "c.csproj"):     nil,
		filepath.Join("root", "sub", "b.txt"): []byte("b"),
	})

	entries, err := fs.ReadDir("root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Immediate children only, sorted: the two files, then the synthesized dir.
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	if entries[0].Name() != "a.txt" || entries[0].IsDir() {
		t.Errorf("entries[0] = %s (dir=%v)", entries[0].Name(), entries[0].IsDir())
	}

	if entries[1].Name() != "c.csproj" || entries[1].IsDir() {
		t.Errorf("entries[1] = %s (dir=%v)", entries[1].Name(), entries[1].IsDir())
	}

	if entries[2].Name() != "sub" || !entries[2].IsDir() {
		t.Errorf("entries[2] = %s (dir=%v), want sub directory", entries[2].Name(), entries[2].IsDir())
	}
}
