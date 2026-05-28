package env

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/ecosystem"
	"github.com/JacobJoergensen/preflight/internal/envfile"
	"github.com/JacobJoergensen/preflight/internal/model"
)

const exampleFile = ".env.example"

func Spec() *ecosystem.Spec {
	return &ecosystem.Spec{
		Name:          "env",
		DisplayName:   "Environment",
		Priority:      10,
		AlwaysPresent: true,
		Check:         check,
	}
}

func check(ctx context.Context, rc ecosystem.RunContext, _ ecosystem.Detection) []model.Message {
	if ctx.Err() != nil {
		return nil
	}

	exampleData, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, exampleFile))
	if err != nil {
		return []model.Message{{Severity: model.SeverityError, Text: "No `" + exampleFile + "` file found. Add one to document required environment variables."}}
	}

	exampleKeys := envfile.ParseKeys(exampleData)

	if len(exampleKeys) == 0 {
		return []model.Message{{Severity: model.SeverityWarning, Text: "`" + exampleFile + "` contains no variable definitions."}}
	}

	envData, err := rc.FS.ReadFile(filepath.Join(rc.WorkDir, ".env"))
	if err != nil {
		return []model.Message{
			{Severity: model.SeverityWarning, Text: "`.env` not found, copy from `" + exampleFile + "` and fill in values."},
			{Severity: model.SeverityError, Text: "Missing environment variables (expected by `" + exampleFile + "`):\n" + strings.Join(exampleKeys, "\n")},
		}
	}

	envSet := make(map[string]struct{})

	for _, key := range envfile.ParseKeys(envData) {
		envSet[key] = struct{}{}
	}

	var missing []string

	for _, key := range exampleKeys {
		if _, ok := envSet[key]; !ok {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return []model.Message{{Severity: model.SeverityError, Text: "Missing environment variables in `.env` (expected by `" + exampleFile + "`):\n" + strings.Join(missing, "\n")}}
	}

	return []model.Message{{Severity: model.SeveritySuccess, Text: "All " + strconv.Itoa(len(exampleKeys)) + " keys from `" + exampleFile + "` are present in `.env`."}}
}
