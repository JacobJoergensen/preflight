package env

import (
	"context"
	"io/fs"
	"slices"
	"testing"
	"time"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/model"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name  string
		files map[string][]byte
		want  []model.Severity
	}{
		{
			name:  "missing example file is an error",
			files: map[string][]byte{},
			want:  []model.Severity{model.SeverityError},
		},
		{
			name:  "example with no variables is a warning",
			files: map[string][]byte{".env.example": []byte("# just a comment\n")},
			want:  []model.Severity{model.SeverityWarning},
		},
		{
			name:  "example present but no .env warns and reports the missing keys",
			files: map[string][]byte{".env.example": []byte("API_KEY=abc\nDB_URL=xyz\n")},
			want:  []model.Severity{model.SeverityWarning, model.SeverityError},
		},
		{
			name: "a key absent from .env is an error",
			files: map[string][]byte{
				".env.example": []byte("API_KEY=abc\nDB_URL=xyz\n"),
				".env":         []byte("API_KEY=local\n"),
			},
			want: []model.Severity{model.SeverityError},
		},
		{
			name: "every key present is a success",
			files: map[string][]byte{
				".env.example": []byte("API_KEY=abc\n"),
				".env":         []byte("API_KEY=local\n"),
			},
			want: []model.Severity{model.SeveritySuccess},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := ecosystem.RunContext{FS: envTestFS{files: tt.files}}
			messages := check(context.Background(), rc, ecosystem.Detection{})

			got := make([]model.Severity, len(messages))

			for i, message := range messages {
				got[i] = message.Severity
			}

			if !slices.Equal(got, tt.want) {
				t.Errorf("severities = %v, want %v", got, tt.want)
			}
		})
	}
}

type envTestFS struct {
	files map[string][]byte
}

func (f envTestFS) ReadFile(name string) ([]byte, error) {
	if data, ok := f.files[name]; ok {
		return data, nil
	}

	return nil, fs.ErrNotExist
}

func (f envTestFS) Stat(name string) (fs.FileInfo, error) {
	if _, ok := f.files[name]; ok {
		return envTestFileInfo{}, nil
	}

	return nil, fs.ErrNotExist
}

func (envTestFS) WriteFile(string, []byte, fs.FileMode) error { return nil }
func (envTestFS) MkdirAll(string, fs.FileMode) error          { return nil }
func (envTestFS) ReadDir(string) ([]fs.DirEntry, error)       { return nil, nil }

type envTestFileInfo struct{}

func (envTestFileInfo) Name() string       { return "" }
func (envTestFileInfo) Size() int64        { return 0 }
func (envTestFileInfo) Mode() fs.FileMode  { return 0 }
func (envTestFileInfo) ModTime() time.Time { return time.Time{} }
func (envTestFileInfo) IsDir() bool        { return false }
func (envTestFileInfo) Sys() any           { return nil }
