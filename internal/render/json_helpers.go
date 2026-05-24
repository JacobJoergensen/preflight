package render

import (
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/JacobJoergensen/preflight/internal/engine/result"
)

func encodeJSON(out io.Writer, v any, pretty bool) error {
	if out == nil {
		return errors.New("json renderer: nil writer")
	}

	encoder := json.NewEncoder(out)

	if pretty {
		encoder.SetIndent("", "  ")
	}

	return encoder.Encode(v)
}

type reportJSON[T any] struct {
	SchemaVersion int           `json:"schemaVersion"`
	StartedAt     time.Time     `json:"startedAt"`
	EndedAt       time.Time     `json:"endedAt"`
	Canceled      bool          `json:"canceled"`
	Items         []T           `json:"items"`
	Projects      []projectJSON `json:"projects,omitempty"`
}

type projectJSON struct {
	RelativePath string `json:"relativePath"`
	Name         string `json:"name,omitempty"`
}

func projectsToJSON(projects []result.Project) []projectJSON {
	if len(projects) == 0 {
		return nil
	}

	out := make([]projectJSON, len(projects))

	for i, project := range projects {
		out[i] = projectJSON{RelativePath: project.RelativePath, Name: project.Name}
	}

	return out
}
