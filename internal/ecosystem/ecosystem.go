package ecosystem

import (
	"context"
	"path/filepath"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs"
	"github.com/JacobJoergensen/preflight/internal/model"
)

type RunContext struct {
	WorkDir string
	FS      fs.FS
	Runner  exec.Runner
	Stream  exec.StreamRunner
}

func (rc RunContext) FileExists(name string) bool {
	_, err := rc.FS.Stat(filepath.Join(rc.WorkDir, name))
	return err == nil
}

type Manager struct {
	Command     string
	DisplayName string
	ConfigFile  string
	LockFile    string
	VersionArgs []string
	InstallArgs []string
	ForceArgs   []string
	Outdated    *OutdatedProbe
	Audit       *AuditProbe
}

type OutdatedProbe struct {
	Tool  string
	Args  []string
	Parse OutdatedParser
}

type AuditProbe struct {
	Tool            string
	Args            []string
	ToolMissingHint string
	Parse           AuditParser
}

type Detection struct {
	Active Manager
}

type (
	CheckFunc      func(ctx context.Context, rc RunContext, detection Detection) []model.Message
	AuditParser    func(stdout string) []model.Finding
	OutdatedParser func(rc RunContext, stdout string) ([]OutdatedPackage, error)
	SignalFunc     func(rc RunContext) []string
	LicenseFunc    func(ctx context.Context, rc RunContext, detection Detection) LicenseResult
)

type Marker struct {
	File     string
	Glob     string // matches when any entry in WorkDir matches this glob (e.g. "*.csproj")
	Contains string
	Manager  string
}

type Spec struct {
	Name        string
	DisplayName string
	Priority    int

	Managers []Manager

	// RuntimeCommands are binaries the Check hook runs beyond the package managers and probe
	// tools (language runtimes like node, php, ruby, the rust compiler, pie). Listing them
	// keeps the command allowlist complete for the gated runner.
	RuntimeCommands []string

	// Check is required.
	Check CheckFunc

	// License lists installed packages and their declared licenses for the licenses
	// command. Ecosystems without a license source leave it nil.
	License LicenseFunc

	// Detect lists the markers that make this ecosystem present, in priority order. When it is
	// empty, the managers' lockfile-then-config files are used. AlwaysPresent overrides both
	// and is set only by env, which runs whenever it is selected.
	Detect        []Marker
	AlwaysPresent bool

	// Optional signals shown in the Project section: version pins, env vars, or a custom override.
	VersionPins  []string
	EnvSignals   []string
	ExtraSignals SignalFunc
}

type OutdatedPackage struct {
	Name    string
	Current string
	Latest  string
}

type FixOptions struct {
	Force      bool
	SkipBackup bool
	DryRun     bool
}

type FixItem struct {
	ScopeID        string
	ManagerCommand string
	ManagerName    string
	Version        string
	Args           []string
	WouldRun       string
	Success        bool
	Error          string
	Output         string
}

type AuditResult struct {
	Skipped      bool
	SkipReason   string
	CommandLine  string
	ExitCode     int
	OK           bool
	SeverityRank int
	Findings     []model.Finding
	Manifest     string // lockfile or config file the audit ran against, for SARIF locations
	Output       string
	Err          error
}

type PackageLicense struct {
	Name    string
	Version string
	License string // SPDX expression as reported by the ecosystem
}

type LicenseResult struct {
	Skipped    bool
	SkipReason string
	Packages   []PackageLicense
	Err        error
}
