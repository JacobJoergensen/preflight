package adapter

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/manifest"
)

func fixWithSelectorCheck(
	ctx context.Context,
	deps Dependencies,
	scopeID string,
	packageType string,
	selectors []string,
	options FixOptions,
) (FixItem, error) {
	packageManager, ok := deps.Loader.DetectPackageManager(packageType)

	if !ok || (!packageManager.ConfigFileExists && !packageManager.LockFileExists) {
		return FixItem{ScopeID: scopeID, Success: true}, nil
	}

	for _, selector := range selectors {
		selectorType, belongsToType := manifest.GetPackageType(selector)

		if belongsToType && selectorType == packageType && !slices.Contains(selectors, packageManager.Command()) {
			return FixItem{
				ScopeID:        scopeID,
				ManagerCommand: packageManager.Command(),
				ManagerName:    packageManager.Name(),
				Success:        false,
				Error:          fmt.Sprintf("requested %s but detected %s", strings.Join(selectors, ","), packageManager.Command()),
			}, nil
		}
	}

	return fixByPackageType(ctx, deps, scopeID, packageType, options)
}

func fixByPackageType(
	ctx context.Context,
	deps Dependencies,
	scopeID string,
	packageType string,
	options FixOptions,
) (FixItem, error) {
	packageManager, ok := deps.Loader.DetectPackageManager(packageType)

	if !ok || (!packageManager.ConfigFileExists && !packageManager.LockFileExists) {
		return FixItem{ScopeID: scopeID, Success: true}, nil
	}

	tool := packageManager.Tool
	version, err := deps.Runner.Run(ctx, tool.Command, tool.VersionArgs...)

	if err != nil {
		return FixItem{
			ScopeID:        scopeID,
			ManagerCommand: tool.Command,
			ManagerName:    packageManager.Name(),
			Success:        false,
			Error:          err.Error(),
		}, nil
	}

	args := slices.Clone(tool.InstallArgs)

	if options.Force {
		args = append(args, tool.ForceArgs...)
	}

	item := FixItem{
		ScopeID:        scopeID,
		ManagerCommand: tool.Command,
		ManagerName:    packageManager.Name(),
		Version:        version,
		Args:           slices.Clone(args),
	}

	if options.DryRun {
		item.WouldRun = fmt.Sprintf("%s %s", tool.Command, strings.Join(args, " "))
		item.Success = true
		return item, nil
	}

	if err := deps.Stream.RunStreaming(ctx, tool.Command, args, os.Stdout, os.Stderr); err != nil {
		item.Success = false
		item.Error = err.Error()
		return item, nil
	}

	item.Success = true
	return item, nil
}
