# Contributing to ssmctl

Thank you for your interest in contributing to ssmctl! This document covers everything you need to get a development environment running, the standards we hold contributions to, and how to submit a pull request.

---

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| [Go](https://golang.org/dl/) | 1.26+ | See `go.mod` for the exact minimum |
| [GNU Make](https://www.gnu.org/software/make/) | any | Used for all dev tasks |
| [golangci-lint](https://golangci-lint.run/usage/install/) | latest | Installed via `make setup` |
| [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html) | v2 | Required only for e2e tests against real AWS |
| [Session Manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) | latest | Required only for `connect` e2e tests |

---

## Getting Started

```bash
# 1. Clone the repository
git clone https://github.com/rhysmcneill/ssmctl.git
cd ssmctl

# 2. Install pre-commit (one-time, per machine)
pip install pre-commit   # or: brew install pre-commit

# 3. Install development tools (golangci-lint, goimports, pre-commit hooks)
make setup

# 4. Verify the build
make build

# 5. Run the unit test suite
make test

# 6. Run the linter
make lint
```

The compiled binary lands at `bin/ssmctl`.

`make setup` also installs the project's [pre-commit](https://pre-commit.com/)
hooks (configured in `.pre-commit-config.yaml`). Hooks run automatically on
every `git commit` and mirror the local Make targets — formatting, `go vet`,
`golangci-lint`, plus light file-hygiene checks (trailing whitespace, final
newlines, merge-conflict markers, accidental large binaries) and a `gosec`
static-security pass. To run them manually across the whole tree:

```bash
pre-commit run --all-files
```

If `pre-commit` isn't on your `PATH` when `make setup` runs, the make target
prints an install hint and continues — you can re-run `make setup` after
installing it.

---

## Available Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary for the current platform |
| `make build-all` | Cross-compile for all supported platforms |
| `make install` | Install the binary to `$GOPATH/bin` |
| `make test` | Run all unit tests |
| `make test-cover` | Run unit tests with a coverage report |
| `make lint` | Run golangci-lint |
| `make fmt` | Run `gofmt` and `goimports` across the repo |
| `make vet` | Run `go vet` |
| `make e2e` | Run CLI smoke tests (no AWS required) |
| `make e2e-aws` | Run full integration tests (real AWS required) |
| `make setup` | Install development tools |
| `make ci` | Run everything CI runs (vet + test + build + e2e) |

---

## Code Style

- Follow standard [Go conventions](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) guide.
- All code must pass `go vet` and `golangci-lint run` without warnings.
- Format code with `gofmt` before committing (`make fmt`).
- Keep functions small and focused. Prefer returning errors over panicking.
- Avoid adding comments that just narrate what the code already clearly says.

---

## Testing Standards

### Unit tests

- Unit tests live alongside source files: `internal/ssm/run_test.go` tests `internal/ssm/run.go`.
- Use the standard `testing` package — no third-party assertion libraries.
- Mock external dependencies (SSM, EC2) using local interface mocks. See `internal/ssm/run_test.go` for the pattern.
- Tests must pass with `go test ./...`.

### E2E / smoke tests

- Smoke tests live in `e2e/` and test the compiled binary. They require no AWS credentials and run in CI on every PR.
- AWS integration tests also live in `e2e/` but are guarded by the `e2e` build tag. Run them with `make e2e-aws` against a real AWS account.

```bash
# Smoke tests only (no AWS)
make e2e

# Full integration suite (requires AWS credentials + a running SSM-enabled instance)
E2E_INSTANCE_ID=i-0123456789abcdef0 make e2e-aws
```

Region, profile, and all other AWS settings are read from the standard environment (`AWS_DEFAULT_REGION`, `AWS_PROFILE`, `~/.aws/config`). Only `E2E_INSTANCE_ID` is test-specific.

### Coverage

Aim to keep unit test coverage above 80 % for packages in `internal/`. Check coverage with:

```bash
make test-cover
```

### Debug Mode Testing

When making changes to AWS SDK interactions, test with the `--debug` flag: 

```bash
ssmctl --debug run i-xxx -- whoami
```

---

## Commit Messages

All commit messages must follow the format:

```
<type>(<scope>): <message>
```

Use the imperative mood in `<message>` (e.g., "add support for..." not "added support for..."). Keep the subject line under 72 characters. Separate the subject from the body with a blank line, and use the body to explain *what* and *why*, not *how*.

### Types

| Type | Purpose |
|------|---------|
| `feat` | New feature or user-facing functionality |
| `fix` | Bug fix |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `test` | Adding or updating tests |
| `chore` | Maintenance, tooling, CI, dependencies, or documentation |
| `perf` | Performance improvement |
| `style` | Code formatting (no logic changes) |
| `ci` | CI/CD configuration changes |
| `release` | Release config updates |
| `infra` | Infrastructure updates |

---

## Pull Request Checklist

Before opening a PR, verify:

- [ ] `make ci` passes locally
- [ ] New behaviour is covered by tests
- [ ] No linter warnings (`make lint`)
- [ ] PR title follows the commit message convention above (validated automatically by CI)

---

## Project Layout

```text
ssmctl/
├── cmd/ssmctl/          # binary entry point — keep thin
├── e2e/                 # end-to-end and smoke tests
├── internal/
│   ├── app/             # application wiring (AWS client setup)
│   ├── cmd/             # Cobra command definitions
│   ├── config/          # flag validation and Config struct
│   ├── output/          # text / JSON output formatting
│   ├── ssm/             # SSM and EC2 API calls + target resolution
│   └── version/         # build-time version variables (set via ldflags)
├── tools/release/       # release-please config
├── Makefile
├── go.mod
└── go.sum
```
