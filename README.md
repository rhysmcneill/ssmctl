# ssmctl

[![CI](https://github.com/rhysmcneill/ssmctl/actions/workflows/ci.yml/badge.svg)](https://github.com/rhysmcneill/ssmctl/actions/workflows/ci.yml) [![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org) [![Version](https://img.shields.io/github/v/tag/rhysmcneill/ssmctl)](https://github.com/rhysmcneill/ssmctl/releases) [![License](https://img.shields.io/github/license/rhysmcneill/ssmctl)](LICENSE) [![Stars](https://img.shields.io/github/stars/rhysmcneill/ssmctl)](https://github.com/rhysmcneill/ssmctl) [![Forks](https://img.shields.io/github/forks/rhysmcneill/ssmctl)](https://github.com/rhysmcneill/ssmctl/forks)

A lightweight CLI for managing AWS SSM connections, remote command execution, and file transfers — designed to feel like a modern SSH/SCP replacement powered by AWS Systems Manager.

---

## Contents

- [Features](#features)
  - [connect](#connect-to-an-instance)
  - [run](#run-a-command)
  - [cp upload](#upload-a-file)
  - [cp download](#download-a-file)
  - [version](#show-version)
- [Targets](#targets)
- [Global Flags](#global-flags)
- [Installation](#installation)
- [Requirements](#requirements)
- [Target OS support](#target-os-support)
- [Design Goals](#design-goals)
- [Project Structure](#project-structure)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [Contributors](#contributors)
- [License](#license)

---

## Features

### Connect to an instance

```bash
ssmctl connect <target>
```

Starts an interactive SSM session. Requires the [AWS Session Manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html).

---

### Run a command

```bash
ssmctl run <target> -- <command>
```

Example:

```bash
ssmctl run web-1 -- uname -a
```

Stdout and stderr are streamed to your terminal. The exit code of the remote command is propagated.

---

### Upload a file

```bash
ssmctl cp ./file.txt <target>:/tmp/file.txt
```

> **Note:** uploads are limited to ~2 MB (SSM `SendCommand` document size).

---

### Download a file

```bash
ssmctl cp <target>:/var/log/app.log ./app.log
```

> **Note:** downloads are limited to ~36 KB (SSM `GetCommandInvocation` output size).

---

### Show version

```bash
ssmctl version
```

---

## Targets

A `<target>` can be:

- An EC2 instance ID — `i-0123456789abcdef0`
- An EC2 Name tag — `web-1`

Resolution strategy:

- Input starting with `i-` is treated as an instance ID directly.
- Everything else is looked up via the EC2 `Name` tag.

---

## Global Flags

```bash
--profile, -p   AWS profile (defaults to AWS_PROFILE env var)
--region,  -r   AWS region
--output,  -o   Output format: text | json  (default: text)
--debug,   -d   Enable debug logging
--timeout, -t   Timeout for remote commands (default: 60s)
```

---

## Installation

### Download a release binary (recommended)

Pre-built binaries for Linux, macOS (Intel + Apple Silicon), and Windows are attached to every [GitHub release](https://github.com/rhysmcneill/ssmctl/releases).

```bash
# Example — adjust version and platform as needed
curl -L https://github.com/rhysmcneill/ssmctl/releases/latest/download/ssmctl-linux-amd64 \
  -o /usr/local/bin/ssmctl && chmod +x /usr/local/bin/ssmctl
```

### Homebrew (macOS / Linux)

```bash
brew tap rhysmcneill/ssmctl
brew install ssmctl
```

---

## Requirements

- AWS credentials configured (environment variables, `~/.aws/credentials`, or an IAM role)
- The target EC2 instance must have the [SSM Agent](https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent.html) installed and running
- For `connect`, the [Session Manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) must be installed locally

---

## Target OS support

| Command | Linux/macOS targets | Windows targets |
|---------|---------------------|-----------------|
| `connect` | Supported | Supported when the Session Manager plugin is installed locally |
| `run` | Supported via `AWS-RunShellScript` | Not currently supported; Windows targets require `AWS-RunPowerShellScript` |
| `cp` | Supported | Not currently supported; transfers rely on POSIX utilities such as `cat` and `base64` |

The `run` and `cp` commands currently build shell commands for POSIX-like
targets. When EC2 metadata identifies a Windows target, these commands return a
clear unsupported-target error instead of running a shell command that would fail
remotely. Use `connect` for Windows targets, or run PowerShell commands through
AWS Systems Manager directly until `ssmctl` gains native Windows command and
transfer support.

---

## Design Goals

- Simple, ergonomic CLI (inspired by `ssh` and `scp`)
- No SSH keys or open inbound ports required
- Built entirely on AWS SSM
- Works with existing AWS credentials and config
- Scriptable via `--output json`

---

## Project Structure

```text
ssmctl/
├── cmd/ssmctl/          # binary entry point
├── e2e/                 # end-to-end tests
├── internal/
│   ├── app/             # application wiring (AWS client setup)
│   ├── cmd/             # Cobra command definitions
│   ├── config/          # flag validation and configuration
│   ├── output/          # text / JSON output formatting
│   ├── ssm/             # SSM and EC2 API calls
│   └── version/         # build-time version variables
├── tools/release/       # release-please configuration
├── go.mod
└── go.sum
```

---

## Roadmap

- [x] `connect` via SSM Session Manager
- [x] `run` command execution via `SendCommand`
- [x] `cp` upload (local → remote)
- [x] `cp` download (remote → local)
- [x] target resolution (instance ID + Name tag)
- [x] structured output (`text` / `json`)
- [x] timeout + context handling
- [x] basic error handling and validation
- [x] Homebrew formula
- [ ] shell completion (`bash`, `zsh`, `fish`)
- [ ] `--output json` support for `connect`

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## License

MIT License

---

## Contributors

Thanks to everyone who has contributed to ssmctl!

[![Contributors](https://contrib.rocks/image?repo=rhysmcneill/ssmctl)](https://github.com/rhysmcneill/ssmctl/graphs/contributors)

---