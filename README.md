# PreFlight

A CLI tool that validates your project dependencies before you run into problems. Checks if everything is installed, fixes what's missing, and runs security audits across package managers.

## Install

```sh
go install github.com/JacobJoergensen/preflight@latest
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
preflight check --pm=npm,composer
preflight check --scope=js,go
preflight check --with-env
```

| Flag | Description |
|------|-------------|
| `--pm`, `-p` | Package managers to check (npm, yarn, pnpm, bun, composer, go, pip, bundle) |
| `--scope` | Scopes to check (js, php, composer, node, go, python, ruby, env) |
| `--with-env` | Also validate `.env` against `.env.example` |
| `--outdated` | Also check for outdated packages |
| `--timeout`, `-t` | Timeout duration (default: 5m) |
| `--json` | Output as JSON |

### fix

Installs missing dependencies.

```sh
preflight fix
preflight fix --pm=npm
preflight fix --dry-run
preflight fix --force
```

| Flag | Description |
|------|-------------|
| `--pm`, `-p` | Package managers to fix |
| `--scope` | Scopes to fix |
| `--force`, `-f` | Force reinstall |
| `--dry-run` | Show what would run without executing |
| `--skip-backup` | Skip lockfile backup |
| `--timeout`, `-t` | Timeout duration (default: 30m) |
| `--json` | Output as JSON |

### audit

Runs native security scanners for each ecosystem.

```sh
preflight audit
preflight audit --scope=js,composer
preflight audit --json
```

| Scope | Tool |
|-------|------|
| js | npm/pnpm/yarn/bun audit |
| composer | composer audit |
| go | govulncheck |
| python | pip-audit |
| ruby | bundle-audit |

| Flag | Description |
|------|-------------|
| `--pm`, `-p` | Package managers to audit |
| `--scope` | Scopes to audit |
| `--timeout`, `-t` | Timeout duration (default: 30m) |
| `--json` | Output as JSON |

### list

Lists all dependencies for the project.

```sh
preflight list
preflight list --pm=composer,go
```

| Flag | Description |
|------|-------------|
| `--pm`, `-p` | Package managers to list |
| `--scope` | Scopes to list |
| `--outdated` | Show outdated packages with version info |
| `--json` | Output as JSON |

### run

Runs a named script from `preflight.yml`.

```sh
preflight run test
preflight run build --dry-run
```

| Flag | Description |
|------|-------------|
| `--dry-run` | Print command without running |
| `--timeout` | Timeout duration (default: 30m) |

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
| `--no-emoji` | Disable emoji output |

## Configuration

Create `preflight.yml` in your project root, or run `preflight init` to generate one.

```yaml
version: 1
profile: default

profiles:
  default:
    check:
      pm: [npm, composer]
      withEnv: true
    fix:
      pm: [npm, composer]
    run:
      scripts:
        test:
          js: "npm test"
        build:
          js: "npm run build"

  ci:
    check:
      scope: [js, composer, go]
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
```

## Scopes vs Package Managers

Use `--scope` for categories, `--pm` for specific tools.

| Scope | Package Managers |
|-------|------------------|
| js | npm, yarn, pnpm, bun |
| composer | composer |
| go | go |
| python | pip, poetry, uv |
| ruby | bundle |
| php | (runtime check only) |
| node | (runtime check only) |
| env | (.env validation) |

You can use either `--scope` or `--pm`, not both.

## Supported Ecosystems

| Ecosystem | Runtime | Package Managers |
|-----------|---------|------------------|
| JavaScript | Node.js | npm, yarn, pnpm, bun |
| PHP | PHP | Composer |
| Go | Go | Go modules |
| Python | Python | pip, Poetry, uv |
| Ruby | Ruby | Bundler |

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. 
