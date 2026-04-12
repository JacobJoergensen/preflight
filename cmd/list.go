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
	managers []string
	scopes   []string
	json     bool
	outdated bool
}

var listOpts listOptions

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all required dependencies for this project",
	Long:    `Lists all dependencies required by this project based on package manager configuration files.`,
	Example: "preflight list --pm=composer,go",
	Aliases: []string{"dependencies", "deps"},
	RunE: func(cmd *cobra.Command, _ []string) error {
		workDir, _ := os.Getwd()
		runner := engine.NewRunner(workDir)

		config, profileName, err := loadPreflightConfig(workDir)

		if err != nil {
			return fmt.Errorf("%slist failed: %w%s", terminal.Red, err, terminal.Reset)
		}

		profile, err := config.ProfileFor(profileName)

		if err != nil {
			return fmt.Errorf("%s%w%s", terminal.Red, err, terminal.Reset)
		}

		scopes := listOpts.scopes
		managers := listOpts.managers

		scopeFromCLI := cmd.Flags().Changed("scope")
		pmFromCLI := cmd.Flags().Changed("pm")

		if profile.List != nil {
			list := profile.List

			if !scopeFromCLI && !pmFromCLI && list.Scope != nil {
				scopes = *list.Scope
			}

			if !scopeFromCLI && !pmFromCLI && list.PM != nil {
				managers = *list.PM
			}
		}

		if len(scopes) > 0 && len(managers) > 0 {
			return fmt.Errorf("%scannot use both --scope and --pm%s", terminal.Red, terminal.Reset)
		}

		report, err := runner.List(cmd.Context(), scopes, managers, listOpts.outdated)

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

	rootCmd.AddCommand(listCmd)
}
