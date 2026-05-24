# Changelog

## Unreleased

## Version 2.0.0-beta.1 (2026-05-24)
- Added .NET (NuGet) ecosystem support: `check`, `audit`, `--outdated`, and `fix` for projects detected via `*.csproj`/`*.fsproj`/`*.vbproj`/`*.sln`, using the native `dotnet` CLI
- Added a `licenses` command that checks dependency licenses against an allow/deny policy (`licenses.allow`/`licenses.deny` in preflight.yml, or `--allow`/`--deny`) across all supported ecosystems (Composer, Rust, JavaScript natively; Go, Python, and Ruby via go-licenses, pip-licenses, and license_finder)
- `audit` now reports individual findings with advisory ID, affected package, severity, and advisory URL instead of only severity counts; `audit --json` adds a `findings` array (schemaVersion 3)
- `audit` can now suppress accepted advisories via `ignoredCves` in preflight.yml or the repeatable `--ignore-cve` flag, matched by CVE/GHSA ID or alias; an ecosystem whose findings are all suppressed passes
- `audit -o sarif` exports findings as SARIF 2.1.0 for upload to GitHub/GitLab code scanning; in SARIF mode findings are reported to code scanning rather than via a non-zero exit, so a later upload step still runs
- Release artifacts now ship cosign signatures, SBOMs, and SLSA build provenance, and npm packages are published with provenance, so installs can be verified
- `check` now offers to run `fix` when it finds missing dependencies in an interactive terminal, via a `y/N` prompt that defaults to no (skipped in CI, with `--quiet`, or `-o json`)
- `init` and `hooks install` now prompt to confirm overwriting an existing file in an interactive terminal instead of requiring `--force` (the `--force`-or-error behavior is unchanged in CI and non-interactive use)
- Added a global `--debug` flag that logs each command run, its exit code, and duration (plus stderr on failure) to stderr
- Added a global `--cwd`/`-C` flag to run PreFlight as if started in another directory
- Monorepo traversal now recognizes Cargo workspaces and uv (Python) workspaces, alongside npm/yarn/pnpm/bun and Go
- Replaced the per-command `--json` flag with `-o`/`--format text|json`
- Replaced the `version` command with a `--version` flag
- Replaced the `--pm` and `--scope` flags with a single `--only` flag
- `-v` is now the shorthand for `--verbose`; print the version with the long `--version` flag
- `check --json` now reports one `messages` array per scope with a `severity` field instead of separate `errors`/`warnings`/`successes` arrays
- `fix --json` now emits camelCase keys and a `schemaVersion`, matching `check --json` and `audit --json`
- Usage and internal errors now exit with code 2, leaving exit code 1 for findings such as missing dependencies, vulnerabilities, or fix failures
- `check` lists installed dependencies by default and collapses large sections to a count; pass `--verbose` to list every dependency
- `check` Project section no longer shows redundant lines: the package manager version (already under Toolchain), static Node scope text, or `<file> exists` and `<manifest> found:` confirmations
- `check` now warns when a project that declares dependencies has no lockfile, since installs are not reproducible without one
- `check` Toolchain line for PHP no longer includes the build date and compiler, only the installed version and required range
- `fix` failures now report the command and exit code instead of a bare `exit status N`
- `hooks install` now writes to the directory from `core.hooksPath` and supports git worktrees, so it works alongside Husky and custom hook setups
- Colored output now turns off automatically when output is not a terminal and honors the `NO_COLOR` and `FORCE_COLOR` environment variables
- `check` no longer falsely reports PHP as not installed, or lists a startup warning as an extension, when PHP prints warnings (such as a failed extension load) before its version banner
- `check`, `fix`, and `audit` no longer hang after a timeout or Ctrl-C when a package manager leaves a child process running
- Removed the `list` command
- Removed the GitHub update check
- Dropped support for the legacy `bun.lockb` lockfile; bun projects are detected via `bun.lock`

## Version 1.6.0 (2026-05-21)
- npm package now installs without a postinstall script, fixing installation under pnpm
- Removed winget packaging, which was never published

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
