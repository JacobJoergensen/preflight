package adapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/exec"
)

func init() {
	Register(NodeModule{})
}

type NodeModule struct{}

func (n NodeModule) Name() string {
	return "node"
}

func (n NodeModule) DisplayName() string {
	return "Node.js"
}

func (n NodeModule) Check(ctx context.Context, deps Dependencies) ([]Message, []Message, []Message) {
	errs := []Message{}
	warns := []Message{}
	succs := []Message{}

	if ctx.Err() != nil {
		return errs, warns, succs
	}

	packageConfig := deps.Loader.LoadPackageConfig()

	if !packageConfig.HasConfig {
		return errs, warns, succs
	}

	if packageConfig.Error != nil {
		warns = append(warns, Message{Text: packageConfig.Error.Error()})
		return errs, warns, succs
	}

	if packageConfig.NodeVersion != "" {
		pin, pinLabel := deps.Loader.ReadNodeVersionPinSource()

		if pin != "" && pinLabel != "" && !nodeEngineSatisfiedByRuntime(pin, packageConfig.NodeVersion) {
			warns = append(warns, Message{Text: fmt.Sprintf(
				"%s pins %s but engines.node is %s — align these for CI vs local.",
				pinLabel,
				strings.TrimSpace(pin),
				strings.TrimSpace(packageConfig.NodeVersion),
			)})
		}

		nodeVersion, err := getNodeVersion(ctx, deps.Runner)

		if err != nil {
			errs = append(errs, Message{Text: fmt.Sprintf("Node is not installed or not on PATH: %v", err)})
			return errs, warns, succs
		}

		versionPrefix := strings.TrimPrefix(strings.Split(nodeVersion, ".")[0], "v")
		feedback := buildNodeEngineFeedback(nodeVersion, packageConfig.NodeVersion, versionPrefix)

		if feedback.ShouldWarnEOL {
			warns = append(warns, Message{Text: feedback.EOLWarning})

			if feedback.ShouldWarnExtra {
				warns = append(warns, Message{Text: feedback.Feedback})
			}
		} else if feedback.ShouldError {
			errs = append(errs, Message{Text: feedback.Feedback})
		} else if feedback.ShouldSuccess {
			succs = append(succs, Message{Text: feedback.Feedback})
		}
	}

	return errs, warns, succs
}

func getNodeVersion(ctx context.Context, runner exec.Runner) (string, error) {
	output, err := runner.Run(ctx, "node", "--version")

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}
