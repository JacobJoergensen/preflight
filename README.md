# PreFlight

A CLI tool that validates your project dependencies before you run into problems. Checks if everything is installed, fixes what's missing, and runs security audits across package managers.

## Install

Go Install:

```sh
go install github.com/JacobJoergensen/preflight@latest
```

Npm / pnpm Install:

```sh
# npm
npm install -g @jacobjoergensen/preflight

# pnpm
pnpm add -g @jacobjoergensen/preflight
```

Or download it from [releases](https://github.com/JacobJoergensen/preflight/releases).

## Get Started

```sh
# Check if dependencies are installed
preflight check

# Fix missing dependencies
preflight fix

# Run security audits
preflight audit
```

## Commands

### check

Validates that all required dependencies are installed.

```sh
preflight check
preflight check --only npm,composer
preflight check --only js,go
preflight check --with-env
```

| Flag | Description |
|------|-------------|
| `--only` | Limit to ecosystems or tools (js, npm, php, composer, node, go, rust, python, ruby, env) |
| `--with-env` | Also validate `.env` against `.env.example` |
| `--outdated` | Also check for outdated packages |
| `--timeout`, `-t` | Timeout duration (default: 5m) |
| `--no-monorepo` | Only check the current directory |
| `--project` | Restrict to sub-projects matching path globs (e.g. `packages/*`) |
| `--format`, `-o` | Output format: text or json (default: text) |

### fix

Installs missing dependencies. Prompts per ecosystem by default and prints a lock file diff after each step.

```sh
preflight fix
preflight fix --only npm
preflight fix --dry-run
preflight fix --yes --no-diff
```

| Flag | Description |
|------|-------------|
| `--only` | Limit to ecosystems or tools |
| `--force`, `-f` | Force reinstall |
| `--dry-run` | Show what would run without executing |
| `--yes`, `-y` | Apply every ecosystem without prompting |
| `--no-diff` | Hide the lock file diff summary |
| `--skip-backup` | Skip lockfile backup |
| `--timeout`, `-t` | Timeout duration (default: 30m) |
| `--no-monorepo` | Only fix the current directory |
| `--project` | Restrict to sub-projects matching path globs |
| `--format`, `-o` | Output format: text or json (default: text) |

### audit

Runs native security scanners for each ecosystem.

```sh
preflight audit
preflight audit --only js,composer
preflight audit -o json
preflight audit -o sarif > preflight.sarif
```

| Scope | Tool |
|-------|------|
| js | npm/pnpm/yarn/bun audit |
| composer | composer audit |
| go | govulncheck |
| rust | cargo-audit |
| python | pip-audit |
| ruby | bundle-audit |

| Flag | Description |
|------|-------------|
| `--only` | Limit to ecosystems or tools |
| `--min-severity` | Minimum severity to report (info, low, moderate, high, critical) |
| `--ignore-cve` | Advisory ID (CVE/GHSA) to suppress; repeatable, merged with `ignoredCves` |
| `--timeout`, `-t` | Timeout duration (default: 30m) |
| `--no-monorepo` | Only audit the current directory |
| `--project` | Restrict to sub-projects matching path globs |
| `--format`, `-o` | Output format: text, json, or sarif (default: text) |

Each finding carries the advisory ID, affected package, severity, and a link. Suppress reviewed, accepted advisories with `--ignore-cve CVE-2023-1234` (repeatable) or the `ignoredCves` list in `preflight.yml`; matching is by CVE/GHSA ID or alias, and an ecosystem whose findings are all suppressed passes.

Export findings to GitHub/GitLab code scanning with SARIF:

```yaml
- run: preflight audit -o sarif > preflight.sarif
- uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: preflight.sarif
```

In SARIF mode, findings are reported to code scanning rather than failing the step, so the upload step always runs (a tool that fails to run still fails the step).

### run

Runs a named script from `preflight.yml`.

```sh
preflight run test
preflight run build --dry-run
```

| Flag | Description |
|------|-------------|
| `--dry-run` | Print command without running |
| `--timeout`, `-t` | Timeout duration (default: 30m) |

### hooks

Manages Git pre-commit hooks.

```sh
preflight hooks install
preflight hooks install --force
preflight hooks install --command "preflight check --with-env"
preflight hooks remove
```

| Flag | Description |
|------|-------------|
| `--force` | Append to existing hook without PreFlight markers |
| `--command` | Custom command to run (default: `preflight check`) |

### init

Generates `preflight.yml` from detected project manifests.

```sh
preflight init
preflight init --force
```

| Flag | Description |
|------|-------------|
| `--force` | Overwrite existing file |

### Global Flags

These work with any command:

| Flag | Description |
|------|-------------|
| `--profile` | Use specific profile from `preflight.yml` |
| `--quiet` | Suppress non-essential output |
| `--no-color` | Disable colored output |
| `--version`, `-v` | Print version, commit, build date, and platform |

Color also turns off automatically when output is not a terminal (piped or redirected), and respects the `NO_COLOR` and `FORCE_COLOR` environment variables.

## Monorepo

`check`, `audit`, and `fix` detect `pnpm-workspace.yaml`, npm/yarn workspaces, and `go.work`, then run per sub-project with aggregated results. If no workspace config is present, preflight scans for directories with project manifests.

Disable with `--no-monorepo`. Narrow the scope with `--project packages/*`.

## Configuration

Create `preflight.yml` in your project root, or run `preflight init` to generate one.

```yaml
version: 1
profile: default

profiles:
  default:
    check:
      only: [npm, composer]
      withEnv: true
    fix:
      only: [npm, composer]
    audit:
      minSeverity: high  # ignore info, low, moderate
      ignoredCves: [CVE-2023-1234]  # suppress reviewed, accepted advisories
    run:
      scripts:
        test:
          js: "npm test"
        build:
          js: "npm run build"

  ci:
    check:
      only: [js, composer, go]
    audit:
      minSeverity: critical  # only fail on critical
```

### Profile Resolution

Priority (highest wins):
1. `--profile` flag
2. `PREFLIGHT_PROFILE` environment variable
3. `profile:` field in `preflight.yml`
4. `default`

### Scripts

Each script targets exactly one package manager:

```yaml
run:
  scripts:
    test:
      js: "npm test"           # runs: npm test
    lint:
      composer: "phpstan"      # runs: composer phpstan
    vet:
      go: "go vet ./..."       # runs: go vet ./...
    spec:
      ruby: "rspec"            # runs: bundle exec rspec
    check:
      python: "pytest"         # runs: poetry run pytest (or pip)
    test-rust:
      rust: "test --all"       # runs: cargo test --all
```

## Selecting ecosystems and tools

By default preflight auto-detects every ecosystem present in the project. Use `--only` to narrow to specific ones. Each value is either an ecosystem or a specific tool; naming a tool also asserts the project uses it.

| Value | Selects |
|-------|---------|
| js | JavaScript (npm, yarn, pnpm, bun) |
| npm, yarn, pnpm, bun | JavaScript, pinned to that tool |
| composer | PHP Composer |
| go | Go modules |
| rust, cargo | Rust |
| python, pip, poetry, uv, pdm, pipenv | Python |
| ruby, bundle | Ruby |
| php, node | runtime check only |
| env | .env validation |

## Supported Ecosystems

| Ecosystem | Runtime | Package Managers |
|-----------|---------|------------------|
| JavaScript | Node.js | npm, yarn, pnpm, bun |
| PHP | PHP | Composer |
| Go | Go | Go modules |
| Rust | Rust | Cargo |
| Python | Python | pip, Poetry, uv |
| Ruby | Ruby | Bundler |

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. 
