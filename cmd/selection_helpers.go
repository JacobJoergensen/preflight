package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func resolveScopeAndPM(cmd *cobra.Command, cliScopes, cliPM []string, profileScopes, profilePM *[]string) ([]string, []string) {
	if cmd.Flags().Changed("scope") || cmd.Flags().Changed("pm") {
		return cliScopes, cliPM
	}

	scopes := cliScopes
	managers := cliPM

	if profileScopes != nil {
		scopes = *profileScopes
	}

	if profilePM != nil {
		managers = *profilePM
	}

	return scopes, managers
}

func validateScopeAndPM(scopes, managers []string) error {
	if len(scopes) > 0 && len(managers) > 0 {
		return fmt.Errorf("%scannot use both --scope and --pm%s", terminal.Red, terminal.Reset)
	}

	return nil
}
