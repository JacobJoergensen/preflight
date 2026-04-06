package hooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	MarkerBegin        = "# BEGIN PREFLIGHT"
	MarkerEnd          = "# END PREFLIGHT"
	DefaultHookCommand = "preflight check"
)

var ErrHookExists = errors.New("pre-commit hook already exists without a PreFlight block (use --force to append)")

func GitRoot(workDir string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", "--show-toplevel")
	cmd.Dir = workDir

	output, err := cmd.Output()

	if err != nil {
		return "", fmt.Errorf("not a git repository (or git not in PATH): %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func validatePreCommitPath(path string) error {
	abs, err := filepath.Abs(filepath.Clean(path))

	if err != nil {
		return fmt.Errorf("pre-commit path: %w", err)
	}

	if filepath.Base(abs) != "pre-commit" {
		return fmt.Errorf("pre-commit hook path must end with %q", "pre-commit")
	}

	hooksDir := filepath.Dir(abs)

	if filepath.Base(hooksDir) != "hooks" {
		return fmt.Errorf("pre-commit must live under %q", filepath.Join(".git", "hooks"))
	}

	gitDir := filepath.Dir(hooksDir)

	if filepath.Base(gitDir) != ".git" {
		return fmt.Errorf("pre-commit must live under %q", filepath.Join(".git", "hooks"))
	}

	return nil
}

func writeHookFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	if err := os.Chmod(path, 0o700); err != nil { // #nosec G302
		return err
	}

	return nil
}

func PreCommitPath(workDir string) (string, error) {
	root, err := GitRoot(workDir)

	if err != nil {
		return "", err
	}

	return filepath.Join(root, ".git", "hooks", "pre-commit"), nil
}

func RemovePreflightBlock(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inBlock := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == MarkerBegin {
			inBlock = true
			continue
		}

		if trimmedLine == MarkerEnd {
			inBlock = false
			continue
		}

		if !inBlock {
			result = append(result, line)
		}
	}

	return strings.TrimRight(strings.Join(result, "\n"), "\n")
}

func FormatBlock(command string) string {
	command = strings.TrimSpace(command)

	if command == "" {
		command = DefaultHookCommand
	}

	var builder strings.Builder

	builder.WriteString(MarkerBegin)
	builder.WriteString("\n")
	builder.WriteString(command)
	builder.WriteString(" || exit 1\n")
	builder.WriteString(MarkerEnd)

	return builder.String()
}

func Install(preCommitPath, command string, force bool) error {
	if err := validatePreCommitPath(preCommitPath); err != nil {
		return err
	}

	command = strings.TrimSpace(command)

	if command == "" {
		command = DefaultHookCommand
	}

	block := FormatBlock(command)
	existingData, readErr := os.ReadFile(preCommitPath) // #nosec G304 - path checked by validatePreCommitPath

	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read pre-commit: %w", readErr)
	}

	existingContent := string(existingData)

	if readErr == nil && strings.TrimSpace(existingContent) != "" && !strings.Contains(existingContent, MarkerBegin) && !force {
		return ErrHookExists
	}

	stripped := RemovePreflightBlock(existingContent)

	var finalContent string

	if strings.TrimSpace(stripped) == "" {
		finalContent = "#!/bin/sh\n" + block + "\n"
	} else {
		finalContent = strings.TrimRight(stripped, "\n") + "\n\n" + block + "\n"
	}

	if err := writeHookFile(preCommitPath, []byte(finalContent)); err != nil {
		return fmt.Errorf("write pre-commit: %w", err)
	}

	return nil
}

func Remove(preCommitPath string) error {
	if err := validatePreCommitPath(preCommitPath); err != nil {
		return err
	}

	data, err := os.ReadFile(preCommitPath) // #nosec G304 - path checked by validatePreCommitPath

	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("pre-commit hook not found: %s", preCommitPath)
		}

		return fmt.Errorf("read pre-commit: %w", err)
	}

	if !bytes.Contains(data, []byte(MarkerBegin)) {
		return fmt.Errorf("no PreFlight block in %s", preCommitPath)
	}

	remainingContent := RemovePreflightBlock(string(data))

	if strings.TrimSpace(remainingContent) == "" {
		if err := os.Remove(preCommitPath); err != nil {
			return fmt.Errorf("remove empty pre-commit: %w", err)
		}

		return nil
	}

	if err := writeHookFile(preCommitPath, []byte(strings.TrimRight(remainingContent, "\n")+"\n")); err != nil {
		return fmt.Errorf("write pre-commit: %w", err)
	}

	return nil
}
