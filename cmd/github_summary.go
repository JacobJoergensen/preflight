package cmd

import (
	"fmt"
	"io"
	"os"
)

func writeGitHubSummary(render func(io.Writer) error) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")

	if path == "" {
		return
	}

	// #nosec G304,G703 - path is read from GITHUB_STEP_SUMMARY, set by the GitHub Actions runner; we never open arbitrary user input here
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)

	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open GITHUB_STEP_SUMMARY: %v\n", err)
		return
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not close GITHUB_STEP_SUMMARY: %v\n", closeErr)
		}
	}()

	if err := render(file); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write GITHUB_STEP_SUMMARY: %v\n", err)
	}
}
