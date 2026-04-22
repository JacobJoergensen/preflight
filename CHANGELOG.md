# Changelog

## Unreleased
- Added a live progress spinner to `check` and `audit`
- Trimmed PHP toolchain output in `check` to match other ecosystems, only the version line remains
- `check` now errors consistently when a required runtime is missing

## Version 1.2.1 (2026-04-22)
- Rewrote the `version` command with a cleaner layout, commit and build date, and a release URL on available updates

## Version 1.2.0 (2026-04-22)
- Rewrote the `fix` command with a redesigned output, interactive per-ecosystem approval, a live progress spinner, and captured tool output surfaced on failure
- Added a lock file diff summary to the `fix` command, rendered by default; use `--no-diff` to hide it
- Added GitHub Actions step summary output: `check`, `audit`, and `fix` append a Markdown report to `$GITHUB_STEP_SUMMARY` when the env var is set
- Added monorepo support to `check`, `audit`, `list`, and `fix`: detects workspace configs (pnpm, npm, go.work) or falls back to scanning for manifests, with `--no-monorepo` to disable and `--project` to filter
- Changed fix lock-file backups in monorepo mode: each subproject writes its own `.preflight/backups/<timestamp>/` tree; failures in one project no longer abort the rest
- Changed `CheckReport` (schemaVersion 9) and `AuditReport` (schemaVersion 2) JSON shapes: items now carry a `project` field, and `CheckReport` moves outdated packages from a top-level map onto each item
- Changed `DependencyReport` (list command) JSON shape (schemaVersion 1): parallel per-adapter maps replaced by a flat `items` array with a `project` field on each entry

## Version 1.1.0 (2026-04-10)
- Added –outdated flag to `check` and `list` commands to surface packages with available updates
- Added `--min-severity` flag to the `audit` command
- Added `minSeverity` config option for audit command
- Audit output now only includes ecosystems present in the project

## Version 1.0.0 (2026-04-06)
- Initial release