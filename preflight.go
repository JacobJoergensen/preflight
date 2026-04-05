package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/JacobJoergensen/preflight/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		if !errors.Is(err, cmd.ErrSilentFailure) {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}

		os.Exit(1)
	}
}
