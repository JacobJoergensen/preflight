# Changelog

## Unreleased
- Replaced the `version` command with a `--version` flag
- Removed the GitHub update check
- Removed the `list` command
- Replaced the `--pm` and `--scope` flags with a single `--only` flag
- Colored output now turns off automatically when output is not a terminal and honors the `NO_COLOR` and `FORCE_COLOR` environment variables
- Usage and internal errors now exit with code 2, leaving exit code 1 for findings such as missing dependencies, vulnerabilities, or fix failures

## Version 1.5.0 (2026-05-19)
- `audit` now uses native `uv audit` for uv projects, removing the need to install `pip-audit` separately (requires uv 0.11.15 or newer)
- `audit` now uses native `yarn npm audit` for yarn 4 (Berry) projects; yarn 1 is skipped (incompatible JSON output)
- `check` and `list` now use native `poetry show` for Poetry projects instead of shelling pip through `poetry run` (requires Poetry 2.2.0 or newer)
- `check` and `list` now use native `pdm list` and `pdm outdated` for PDM projects instead of shelling pip through `pdm run`
- `check` now lists PIE-managed PHP extensions via `pie show` for accurate detection across all PIE installation methods
- `check` and `fix` now report the correct lockfile name for legacy bun projects using `bun.lockb`
- `check` now suggests `cargo build` instead of `cargo fetch` for missing Rust crates
- Fixed Rust audit severity classification: vulnerabilities now correctly bucket by CVSS score (critical/high/moderate/low) instead of all reporting as info
- Fixed `check --outdated` and `list --outdated` counting npm optional dependencies skipped due to platform mismatch
- Fixed `check --outdated` and `list --outdated` silently reporting zero outdated packages for bun and yarn projects (neither produces npm-shape JSON for `outdated`)
- Fixed `check --outdated` and `list --outdated` silently swallowing real cargo errors for Rust projects when stdout was empty

## Version 1.4.0 (2026-05-14)
- Added Rust ecosystem support: `check`, `list`, `audit`, `fix`, and `run` now recognize Cargo projects via `Cargo.toml`, verify direct and dev dependencies against `Cargo.lock`, dispatch `rust:` script targets to `cargo`, surface outdated crates via `cargo outdated` (when installed), and run security audits via `cargo audit` (when installed)
- `list` now splits dev dependencies into their own section under each ecosystem, matching the layout used by `check`
- `list` now shows optional dependencies (`optionalDependencies` for npm, `suggest` for Composer, non-dev extras for PEP 621 pyproject.toml, `optional = true` markers for Poetry) in a separate section under each ecosystem
- `check` now reports npm `optionalDependencies` in a dedicated section and warns when one targeting the host platform is not installed (packages with mismatched OS/CPU tokens in their name are skipped)
- `check` now shows a count of npm optional packages skipped due to platform mismatch under the Optional dependencies section
- `check --outdated` and `list --outdated` now include outdated optional dependencies for npm and Python
- `check` no longer reports Poetry dependencies marked `optional = true` as missing when not installed
- `check` now supports Yarn Plug'n'Play projects (yarn berry default): when `.pnp.cjs` or `.pnp.loader.mjs` is present, dependency installation status is verified from `yarn.lock` instead of scanning `node_modules/`
- `fix` now reports lockfile diffs for `yarn.lock` alongside the other supported lockfiles
- Fixed `check` silently treating missing npm packages and Composer dependencies as installed

## Version 1.3.0 (2026-04-22)
- Added a live progress spinner to `check` and `audit`
- Trimmed PHP toolchain output in `check` to match other ecosystems
- `check` now errors consistently when a required runtime is missing
- `list` now exits with an error when canceled mid-run

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
