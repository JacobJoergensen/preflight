package adapter

import (
	"context"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs"
)

type pieConfig struct {
	IsInstalled bool
	Extensions  []string
	Error       error
}

func loadpieConfig(ctx context.Context, runner exec.Runner, fsys fs.FS) pieConfig {
	config := pieConfig{}

	invocation := findPIEInvocation(ctx, runner, fsys)

	if invocation == nil {
		return config
	}

	config.IsInstalled = true

	extensions, err := getPIEExtensions(ctx, runner, invocation)
	if err != nil {
		config.Error = err
		return config
	}

	slices.Sort(extensions)
	config.Extensions = extensions

	return config
}

func findPIEInvocation(ctx context.Context, runner exec.Runner, fsys fs.FS) []string {
	if _, err := runner.Run(ctx, "pie", "--version"); err == nil {
		return []string{"pie"}
	}

	if _, err := fsys.Stat("./pie.phar"); err == nil {
		return []string{"php", "./pie.phar"}
	}

	return nil
}

func getPIEExtensions(ctx context.Context, runner exec.Runner, invocation []string) ([]string, error) {
	name := invocation[0]
	args := append(invocation[1:], "show")

	output, err := runner.Run(ctx, name, args...)
	if err != nil {
		return nil, err
	}

	return parsePIEShowOutput(output), nil
}

func parsePIEShowOutput(output string) []string {
	var extensions []string

	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)

		if line == "" || !strings.Contains(line, "(from ") {
			continue
		}

		name, _, ok := strings.Cut(line, ":")

		if !ok {
			continue
		}

		name = strings.TrimSpace(name)

		if name != "" {
			extensions = append(extensions, name)
		}
	}

	return extensions
}
