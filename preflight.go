package main

import (
	"os"

	"github.com/JacobJoergensen/preflight/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
