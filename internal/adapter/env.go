package adapter

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/envfile"
)

func init() {
	Register(EnvModule{})
}

const envExampleFile = ".env.example"

type EnvModule struct{}

func (EnvModule) Name() string {
	return "env"
}

func (EnvModule) DisplayName() string {
	return "Environment"
}

func (e EnvModule) Check(ctx context.Context, deps Dependencies) ([]Message, []Message, []Message) {
	errs := []Message{}
	warns := []Message{}
	succs := make([]Message, 0, 1)

	if ctx.Err() != nil {
		return errs, warns, succs
	}

	base := deps.Loader.WorkDir
	examplePath := filepath.Join(base, envExampleFile)

	exampleData, err := deps.FS.ReadFile(examplePath)

	if err != nil {
		errs = append(errs, Message{
			Text: "No `" + envExampleFile + "` file found. Add one to document required environment variables.",
		})

		return errs, warns, succs
	}

	exampleKeys := envfile.ParseKeys(exampleData)

	if len(exampleKeys) == 0 {
		warns = append(warns, Message{
			Text: "`" + envExampleFile + "` contains no variable definitions.",
		})

		return errs, warns, succs
	}

	envPath := filepath.Join(base, ".env")
	envData, envErr := deps.FS.ReadFile(envPath)

	if envErr != nil {
		warns = append(warns, Message{
			Text: "`.env` not found, copy from `" + envExampleFile + "` and fill in values.",
		})

		errs = append(errs, Message{
			Text: "Missing environment variables (expected by `" + envExampleFile + "`): " + strings.Join(exampleKeys, ", "),
		})

		return errs, warns, succs
	}

	envKeys := envfile.ParseKeys(envData)
	envSet := make(map[string]struct{}, len(envKeys))

	for _, key := range envKeys {
		envSet[key] = struct{}{}
	}

	var missing []string

	for _, key := range exampleKeys {
		if _, ok := envSet[key]; !ok {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		errs = append(errs, Message{
			Text: "Missing environment variables in `.env` (expected by `" + envExampleFile + "`): " + strings.Join(missing, ", "),
		})

		return errs, warns, succs
	}

	succs = append(succs, Message{
		Text: "All " + strconv.Itoa(len(exampleKeys)) + " keys from `" + envExampleFile + "` are present in `.env`.",
	})

	return errs, warns, succs
}
