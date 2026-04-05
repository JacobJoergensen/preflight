package adapter

import (
	"context"
	"errors"
	"fmt"
	goexec "os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JacobJoergensen/preflight/internal/exec"
	"github.com/JacobJoergensen/preflight/internal/fs"
)

var piePharPaths = []string{
	"./pie.phar",
	"/usr/local/bin/pie.phar",
	"/usr/bin/pie.phar",
}

type PIEConfig struct {
	IsInstalled bool
	Extensions  []string
	PharPath    string
	Error       error
}

func loadPIEConfig(ctx context.Context, runner exec.Runner, fsys fs.FS) PIEConfig {
	config := PIEConfig{}
	config.IsInstalled = checkPIEInstalled(ctx, runner, fsys)

	if !config.IsInstalled {
		return config
	}

	pharPath, err := findPIEPharPath(ctx, fsys)

	if err != nil {
		config.Error = fmt.Errorf("could not locate pie.phar: %w", err)
		return config
	}

	config.PharPath = pharPath
	extensionsMap := getPIEExtensions(ctx, runner, pharPath)

	for ext := range extensionsMap {
		if ext == "" || ext == "Core" || ext == "standard" ||
			ext == "[PHP Modules]" || ext == "[Zend Modules]" {
			continue
		}

		config.Extensions = append(config.Extensions, ext)
	}

	slices.Sort(config.Extensions)

	return config
}

func checkPIEInstalled(ctx context.Context, runner exec.Runner, fsys fs.FS) bool {
	if _, err := runner.Run(ctx, "pie", "--version"); err == nil {
		return true
	}

	for _, path := range piePharPaths {
		if _, err := fsys.Stat(path); err == nil {
			return true
		}
	}

	return false
}

func findPIEPharPath(_ context.Context, fsys fs.FS) (string, error) {
	for _, path := range piePharPaths {
		if _, err := fsys.Stat(path); err == nil {
			return path, nil
		}
	}

	piePath, err := goexec.LookPath("pie")

	if err == nil && piePath != "" {
		if filepath.Ext(piePath) == ".phar" {
			return piePath, nil
		}

		pharPath := filepath.Join(filepath.Dir(piePath), "pie.phar")

		if _, err := fsys.Stat(pharPath); err == nil {
			return pharPath, nil
		}
	}

	return "", errors.New("could not find pie.phar")
}

func getPIEExtensions(ctx context.Context, runner exec.Runner, pharPath string) map[string]struct{} {
	extensions := make(map[string]struct{})

	if output, err := runner.Run(ctx, "pie", "-m"); err == nil {
		for ext := range strings.SplitSeq(output, "\n") {
			if ext = strings.TrimSpace(ext); ext != "" {
				extensions[ext] = struct{}{}
			}
		}
	}

	if pharPath == "" {
		return extensions
	}

	metadataScript := fmt.Sprintf(`
		try {
			$phar = new Phar('%s');
			$meta = $phar->getMetadata();
			if (isset($meta['extensions'])) echo implode("\n", $meta['extensions']);
			if (isset($meta['xdebug']) || isset($meta['extensions']['xdebug'])) echo "\nxdebug";
		} catch (Exception $e) { exit(1); }
	`, pharPath)

	if output, err := runner.Run(ctx, "php", "-r", metadataScript); err == nil {
		for ext := range strings.SplitSeq(output, "\n") {
			if ext = strings.TrimSpace(ext); ext != "" {
				extensions[ext] = struct{}{}
			}
		}
	}

	scanScript := fmt.Sprintf(`
		try {
			$phar = new Phar('%s');
			foreach (new RecursiveIteratorIterator($phar) as $file) {
				$p = $file->getPathname();
				if (strpos($p, 'xdebug') !== false) echo "xdebug\n";
				if (preg_match('/extensions\/([a-zA-Z0-9_-]+)\//', $p, $m)) echo $m[1] . "\n";
			}
		} catch (Exception $e) { exit(1); }
	`, pharPath)

	if output, err := runner.Run(ctx, "php", "-r", scanScript); err == nil {
		for ext := range strings.SplitSeq(output, "\n") {
			if ext = strings.TrimSpace(ext); ext != "" {
				extensions[ext] = struct{}{}
			}
		}
	}

	for _, ext := range []string{"xdebug", "opcache", "pcov"} {
		checkScript := fmt.Sprintf(`
			try {
				$phar = new Phar('%s');
				if ($phar->offsetExists('extensions/%s') ||
					$phar->offsetExists('ext/%s') ||
					$phar->offsetExists('%s')) echo "found";
			} catch (Exception $e) { exit(1); }
		`, pharPath, ext, ext, ext)

		if output, err := runner.Run(ctx, "php", "-r", checkScript); err == nil && strings.Contains(output, "found") {
			extensions[ext] = struct{}{}
		}
	}

	return extensions
}
