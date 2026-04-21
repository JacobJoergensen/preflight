# Changelog

## Unreleased
- Rewrote the `fix` command with a redesigned output, interactive per-ecosystem approval, a live progress spinner, and captured tool output surfaced on failure
- Added a lock file diff summary to the `fix` command, rendered by default; use `--no-diff` to hide it
- Added GitHub Actions step summary output: `check`, `audit`, and `fix` append a Markdown report to `$GITHUB_STEP_SUMMARY` when the env var is set
- Added monorepo traversal to the `check`, `audit`, `list`, and `fix` commands: detects pnpm-workspace.yaml, package.json workspaces, and go.work and runs each command per sub-project with aggregated results; `--no-monorepo` disables, `--project` filters by path glob
- Added filesystem walk fallback for monorepo discovery: when no workspace config declares sub-projects, preflight scans up to four levels deep for directories with project manifests (package.json, composer.json, go.mod, pyproject.toml, Gemfile), skipping node_modules, vendor, build output, and dot-directories
- Added a project-scoped `P` key to the interactive fix approver: apply every remaining ecosystem in the current project and resume prompting on the next one (monorepo mode only)
- Changed fix lock-file backups in monorepo mode: each sub-project writes its own `.preflight/backups/<timestamp>/` tree; failures in one project no longer abort the rest
- Changed `CheckReport` JSON shape (schemaVersion 9): outdated packages moved from a top-level map onto each item, and per-project metadata is exposed when traversal runs
- Changed `AuditReport` JSON shape (schemaVersion 2): items gained a `project` field and per-project metadata is exposed when traversal runs
- Changed `DependencyReport` (list command) JSON shape (schemaVersion 1): parallel per-adapter maps replaced by a flat `items` array with a `project` field on each entry

## Version 1.1.0 (2026-04-10)
- Added –outdated flag to `check` and `list` commands to surface packages with available updates
- Added `--min-severity` flag to the `audit` command
- Added `minSeverity` config option for audit command
- Audit output now only includes ecosystems present in the project

## Version 1.0.0 (2026-04-06)
* Initial release