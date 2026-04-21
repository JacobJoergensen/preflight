package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/engine"
	"github.com/JacobJoergensen/preflight/internal/engine/result"
	"github.com/JacobJoergensen/preflight/internal/render"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

type listOptions struct {
	managers     []string
	scopes       []string
	json         bool
	outdated     bool
	noMonorepo   bool
	projectGlobs []string
}

var listOpts listOptions

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all required dependencies for this project",
	Long:    `Lists all dependencies required by this project based on package manager configuration files.`,
	Example: "preflight list --pm=composer,go",
	Aliases: []string{"dependencies", "deps"},
	RunE: func(cmd *cobra.Command, _ []string) error {
		workDir, err := os.Getwd()

		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		runner := engine.NewRunner(workDir)

		config, profileName, err := loadPreflightConfig(workDir)

		if err != nil {
			return fmt.Errorf("%slist failed: %w%s", terminal.Red, err, terminal.Reset)
		}

		profile, err := config.ProfileFor(profileName)

		if err != nil {
			return fmt.Errorf("%s%w%s", terminal.Red, err, terminal.Reset)
		}

		var profileScope, profilePM *[]string

		if profile.List != nil {
			profileScope = profile.List.Scope
			profilePM = profile.List.PM
		}

		scopes, managers := resolveScopeAndPM(cmd, listOpts.scopes, listOpts.managers, profileScope, profilePM)

		if err := validateScopeAndPM(scopes, managers); err != nil {
			return err
		}

		report, err := runner.List(cmd.Context(), scopes, managers, listOpts.outdated, listOpts.noMonorepo, listOpts.projectGlobs)

		if err != nil {
			return fmt.Errorf("%slist failed: %w%s", terminal.Red, err, terminal.Reset)
		}

		if err := renderList(report, listOpts.json); err != nil {
			return err
		}

		return nil
	},
}

func renderList(report result.DependencyReport, jsonOutput bool) error {
	if jsonOutput {
		return render.JSONListRenderer{Out: os.Stdout}.Render(report)
	}

	return render.TTYListRenderer{}.Render(report)
}

func init() {
	listCmd.Flags().StringSliceVarP(
		&listOpts.managers,
		"pm",
		"p",
		[]string{},
		"Tools or scopes to list (aliases: npm,yarn,pnpm,bun → js)",
	)

	listCmd.Flags().StringSliceVar(
		&listOpts.scopes,
		"scope",
		[]string{},
		"Scopes to list (comma-separated: js,composer,go,python,ruby)",
	)

	listCmd.Flags().BoolVar(
		&listOpts.json,
		"json",
		false,
		"Output results as JSON",
	)

	listCmd.Flags().BoolVar(
		&listOpts.outdated,
		"outdated",
		false,
		"Show outdated packages with version info",
	)

	listCmd.Flags().BoolVar(
		&listOpts.noMonorepo,
		"no-monorepo",
		false,
		"Disable monorepo traversal, list only the current directory",
	)

	listCmd.Flags().StringSliceVar(
		&listOpts.projectGlobs,
		"project",
		[]string{},
		"Restrict monorepo traversal to projects matching these path globs (comma-separated, e.g. packages/*)",
	)

	rootCmd.AddCommand(listCmd)
}
