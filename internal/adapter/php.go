package adapter

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/semver"
	"github.com/JacobJoergensen/preflight/internal/terminal"
)

func init() {
	Register(PhpModule{})
}

type PhpModule struct{}

func (p PhpModule) Name() string {
	return "php"
}

func (p PhpModule) DisplayName() string {
	return "PHP"
}

type extensionInfo struct {
	Name      string
	Source    string
	IsWarning bool
	Warning   string
}

var deprecatedExtensions = map[string]struct{}{
	"imap": {}, "mysql": {}, "recode": {}, "statistics": {}, "wddx": {}, "xml-rpc": {},
}

var experimentalExtensions = map[string]struct{}{
	"gmagick": {}, "imagemagick": {}, "mqseries": {}, "parle": {}, "rnp": {},
	"svm": {}, "svn": {}, "ui": {}, "omq": {},
}

var (
	phpVersionRegex = regexp.MustCompile(`PHP (\d+\.\d+\.\d+)`)
	phpBuildRegex   = regexp.MustCompile(`\(built: ([^)]+)\) \((.*?)\)`)
)

func (p PhpModule) Check(ctx context.Context, deps Dependencies) ([]Message, []Message, []Message) {
	errs := []Message{}
	warns := []Message{}
	succs := []Message{}

	if ctx.Err() != nil {
		return errs, warns, succs
	}

	if !deps.Loader.HasComposerPHPContext() {
		return errs, warns, succs
	}

	composerConfig := deps.Loader.LoadComposerConfig()

	if composerConfig.Error != nil {
		errs = append(errs, Message{Text: fmt.Sprintf("Failed to read composer.json: %v", composerConfig.Error)})
		return errs, warns, succs
	}

	phpVersion, buildDate, vcVersion, err := getPhpVersion(ctx, deps.Runner)

	if err != nil {
		warns = append(warns, Message{Text: fmt.Sprintf("Could not run PHP: %v", err)})

		return errs, warns, succs
	}

	if composerConfig.PHPVersion != "" {
		errs, warns, succs = validatePhpVersion(phpVersion, buildDate, vcVersion, composerConfig.PHPVersion, errs, warns, succs)
	}

	if binary, memoryLimit, maxExecution, iniErr := getPhpCLIIni(ctx, deps.Runner); iniErr != nil {
		warns = append(warns, Message{Text: fmt.Sprintf("Could not read PHP CLI binary/ini: %v", iniErr)})
	} else {
		if binary != "" {
			succs = append(succs, Message{Text: fmt.Sprintf("• PHP CLI binary: %s%s", terminal.Reset, binary)})
		} else {
			succs = append(succs, Message{Text: "• PHP CLI binary: (empty PHP_BINARY — check your install)"})
		}

		succs = append(succs, Message{Text: fmt.Sprintf(
			"• PHP CLI ini: memory_limit=%s, max_execution_time=%s",
			memoryLimit,
			maxExecution,
		)})
	}

	installedExtensions, err := getPhpExtensions(ctx, deps.Runner)

	if err != nil {
		errs = append(errs, Message{Text: fmt.Sprintf("Failed to check PHP extensions: %v", err)})
		return errs, warns, succs
	}

	pieConfig := loadPIEConfig(ctx, deps.Runner, deps.FS)
	extensionSources := make(map[string]string)

	for ext := range installedExtensions {
		extensionSources[ext] = "php"
	}

	pieExtensions := make(map[string]struct{})

	if semver.MatchVersionConstraint(phpVersion, ">=8.4") && pieConfig.IsInstalled {
		for _, ext := range pieConfig.Extensions {
			pieExtensions[ext] = struct{}{}
			extensionSources[ext] = "pie"
		}
	}

	extensionsToShow := make([]extensionInfo, 0, len(pieExtensions)+len(composerConfig.PHPExtensions))

	for ext := range pieExtensions {
		if ext == "" || ext == "Core" || ext == "standard" ||
			ext == "[PHP Modules]" || ext == "[Zend Modules]" {
			continue
		}

		extensionsToShow = append(extensionsToShow, buildExtensionInfo(ext, "pie"))
	}

	for _, ext := range composerConfig.PHPExtensions {
		if slices.ContainsFunc(extensionsToShow, func(info extensionInfo) bool {
			return info.Name == ext
		}) {
			continue
		}

		if source, exists := extensionSources[ext]; exists {
			extensionsToShow = append(extensionsToShow, buildExtensionInfo(ext, source))
			continue
		}

		if semver.MatchVersionConstraint(phpVersion, ">=8.4") {
			if altExt := findPdoAlternative(ext, extensionSources); altExt != "" {
				extensionsToShow = append(extensionsToShow, extensionInfo{
					Name:      ext,
					Source:    "php",
					IsWarning: false,
					Warning:   fmt.Sprintf("%s(%s)%s", terminal.Gray, altExt, terminal.Reset),
				})
				continue
			}
		}

		errs = append(errs, Message{Text: fmt.Sprintf("Missing extension %s%s, Please enable it!", terminal.Reset, ext)})
	}

	slices.SortFunc(extensionsToShow, func(a, b extensionInfo) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	for _, extInfo := range extensionsToShow {
		errs, warns, succs = appendExtensionFeedback(extInfo, errs, warns, succs)
	}

	return errs, warns, succs
}

func validatePhpVersion(phpVersion, buildDate, vcVersion, requiredVersion string, errs, warns, succs []Message) ([]Message, []Message, []Message) {
	feedback := fmt.Sprintf("Installed %sPHP (%s ⟶ required %s), Built: (%s, %s)", terminal.Reset, phpVersion, requiredVersion, buildDate, vcVersion)
	versionPrefix := strings.Split(phpVersion, ".")[0] + "." + strings.Split(phpVersion, ".")[1]
	versionFeedback := buildVersionFeedback("php", "PHP", phpVersion, requiredVersion, versionPrefix)

	if isEOL("php", versionPrefix) {
		warns = append(warns, Message{Text: fmt.Sprintf("Installed %sPHP (%s ⟶ End-of-Life), Consider upgrading!", terminal.Reset, phpVersion)})

		if versionFeedback.ShouldWarnExtra {
			warns = append(warns, Message{Text: feedback})
		}
	} else if versionFeedback.ShouldError {
		errs = append(errs, Message{Text: feedback})
	} else {
		succs = append(succs, Message{Text: feedback})
	}

	return errs, warns, succs
}

func buildExtensionInfo(ext, source string) extensionInfo {
	info := extensionInfo{
		Name:   ext,
		Source: source,
	}

	if _, deprecated := deprecatedExtensions[ext]; deprecated {
		info.IsWarning = true
		info.Warning = fmt.Sprintf("(%s ⟶ deprecated), Consider removing or replacing it!", ext)
	} else if _, experimental := experimentalExtensions[ext]; experimental {
		info.IsWarning = true
		info.Warning = fmt.Sprintf("(%s ⟶ experimental), Use with caution!", ext)
	}

	return info
}

func findPdoAlternative(ext string, extensionSources map[string]string) string {
	pdoExtensions := map[string][]string{
		"pdo": {"pdo_sqlite", "pdo_mysql", "pdo_pgsql", "pdo_oci", "pdo_odbc", "pdo_firebird"},
	}

	alternatives, isSplitExt := pdoExtensions[ext]

	if !isSplitExt {
		return ""
	}

	for _, altExt := range alternatives {
		if _, exists := extensionSources[altExt]; exists {
			return altExt
		}
	}

	return ""
}

func appendExtensionFeedback(extInfo extensionInfo, errs, warns, succs []Message) ([]Message, []Message, []Message) {
	var feedback string

	if extInfo.Source == "pie" {
		if extInfo.IsWarning {
			feedback = fmt.Sprintf("Installed extension %s%s %s", terminal.Reset, extInfo.Name, extInfo.Warning)
			warns = append(warns, Message{Text: feedback, Nested: true})
		} else {
			feedback = fmt.Sprintf("Installed extension %s%s %s(Pie)%s", terminal.Reset, extInfo.Name, terminal.Yellow, terminal.Reset)
			succs = append(succs, Message{Text: feedback, Nested: true})
		}
	} else {
		if extInfo.IsWarning {
			feedback = fmt.Sprintf("Installed extension %s%s %s", terminal.Reset, extInfo.Name, extInfo.Warning)
			warns = append(warns, Message{Text: feedback, Nested: true})
		} else if extInfo.Warning != "" {
			feedback = fmt.Sprintf("Installed extension %s%s %s", terminal.Reset, extInfo.Name, extInfo.Warning)
			succs = append(succs, Message{Text: feedback, Nested: true})
		} else {
			feedback = fmt.Sprintf("Installed extension %s%s", terminal.Reset, extInfo.Name)
			succs = append(succs, Message{Text: feedback, Nested: true})
		}
	}

	return errs, warns, succs
}

func getPhpCLIIni(ctx context.Context, runner exec.Runner) (binary, memoryLimit, maxExec string, err error) {
	const script = `echo PHP_BINARY, "\n", ini_get("memory_limit"), "\n", ini_get("max_execution_time");`

	output, err := runner.Run(ctx, "php", "-r", script)

	if err != nil {
		return "", "", "", err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) < 3 {
		return "", "", "", fmt.Errorf("unexpected php -r output: %q", strings.TrimSpace(output))
	}

	return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1]), strings.TrimSpace(lines[2]), nil
}

func getPhpVersion(ctx context.Context, runner exec.Runner) (phpVersion, buildDate, vcVersion string, err error) {
	output, err := runner.Run(ctx, "php", "--version")

	if err != nil {
		return "", "", "", fmt.Errorf("failed to run php --version: %w", err)
	}

	lines := strings.Split(output, "\n")

	if len(lines) == 0 {
		return "", "", "", errors.New("unexpected output from php --version")
	}

	if matches := phpVersionRegex.FindStringSubmatch(lines[0]); len(matches) >= 2 {
		phpVersion = matches[1]
	} else {
		return "", "", "", fmt.Errorf("could not parse PHP version from: %s", lines[0])
	}

	if matches := phpBuildRegex.FindStringSubmatch(lines[0]); len(matches) >= 3 {
		buildDate, vcVersion = matches[1], matches[2]
	} else {
		buildDate, vcVersion = "unknown", "unknown"
	}

	return phpVersion, buildDate, vcVersion, nil
}

func getPhpExtensions(ctx context.Context, runner exec.Runner) (map[string]struct{}, error) {
	output, err := runner.Run(ctx, "php", "-m")

	if err != nil {
		return nil, fmt.Errorf("failed to run php -m: %w", err)
	}

	extensions := make(map[string]struct{})

	for ext := range strings.SplitSeq(output, "\n") {
		if trimmed := strings.TrimSpace(ext); trimmed != "" {
			extensions[trimmed] = struct{}{}
		}
	}

	return extensions, nil
}
