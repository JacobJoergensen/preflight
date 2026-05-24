package terminal

import "os"

var Quiet bool

func SetQuiet(enabled bool) {
	Quiet = enabled
}

var Verbose bool

func SetVerbose(enabled bool) {
	Verbose = enabled
}

func ConfigureColor(disabled bool, out *os.File) {
	if !colorEnabled(disabled, out) {
		disableColor()
	}
}

func colorEnabled(disabled bool, out *os.File) bool {
	if disabled {
		return false
	}

	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}

	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	return IsInteractiveTTY(out)
}
