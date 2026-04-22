package terminal

var (
	Bold  = "\033[1m"
	Dim   = "\033[2m"
	Reset = "\033[0m"

	Border = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	Gray   = "\033[90m"
	Green  = "\033[32m"
	Red    = "\033[31m"
	Yellow = "\033[33m"

	CheckMark   = "✓"
	CrossMark   = "✗"
	WarningSign = "⚠"
	Lightning   = "⚡"
)

func DisableColor() {
	Bold = ""
	Dim = ""
	Reset = ""
	Blue = ""
	Cyan = ""
	Gray = ""
	Green = ""
	Red = ""
	Yellow = ""
}
