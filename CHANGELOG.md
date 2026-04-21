# Changelog

## Unreleased
- Rewrote the `fix` command with a redesigned output, interactive per-ecosystem approval, a live progress spinner, and captured tool output surfaced on failure
- Added a lock file diff summary to the `fix` command, rendered by default; use `--no-diff` to hide it
- Added GitHub Actions step summary output: `check`, `audit`, and `fix` append a Markdown report to `$GITHUB_STEP_SUMMARY` when the env var is set

## Version 1.1.0 (2026-04-10)
- Added –outdated flag to `check` and `list` commands to surface packages with available updates
- Added `--min-severity` flag to the `audit` command
- Added `minSeverity` config option for audit command
- Audit output now only includes ecosystems present in the project

## Version 1.0.0 (2026-04-06)
* Initial release