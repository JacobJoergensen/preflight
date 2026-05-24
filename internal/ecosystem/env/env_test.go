package env

import (
	"context"
	"slices"
	"testing"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/fs/memfs"
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
			rc := ecosystem.RunContext{FS: memfs.New(tt.files)}
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
