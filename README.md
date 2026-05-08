# ssmctl

[![CI](https://github.com/rhysmcneill/ssmctl/actions/workflows/ci.yml/badge.svg)](https://github.com/rhysmcneill/ssmctl/actions/workflows/ci.yml) [![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org) [![Version](https://img.shields.io/github/v/tag/rhysmcneill/ssmctl)](https://github.com/rhysmcneill/ssmctl/releases) [![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE) [![Stars](https://img.shields.io/github/stars/rhysmcneill/ssmctl?style=flat)](https://github.com/rhysmcneill/ssmctl) [![Forks](https://img.shields.io/github/forks/rhysmcneill/ssmctl)](https://github.com/rhysmcneill/ssmctl/forks)

A lightweight CLI for managing AWS SSM connections, remote command execution, and file transfers — designed to feel like a modern SSH/SCP replacement powered by AWS Systems Manager.

---

## Contents

- [Features](#features)
  - [list](#list-instances)
  - [connect](#connect-to-an-instance)
  - [forward](#forward-a-port)
  - [run](#run-a-command)
  - [cp upload](#upload-a-file)
  - [cp download](#download-a-file)
  - [cp via S3 (large files)](#large-file-transfers-via-s3)
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

### List instances

```bash
ssmctl list [--filter <substring>] [--platform linux|windows]
```

Lists all EC2 instances managed by AWS Systems Manager. Instance IDs, Name tags, platform, agent version, and ping status are displayed as a table. Use `--output json` for structured output.

```
INSTANCE ID           NAME        PLATFORM   AGENT VERSION   STATUS
i-0123456789abcdef0   web-1       Linux      3.2.2086.0      Online
i-0987654321fedcba0   bastion-1   Linux      3.2.2086.0      Online
i-0aabbccddeeff0011   win-app-1   Windows    3.2.2086.0      Offline
```

Filter examples:

```bash
# Substring match on Name tag
ssmctl list --filter web

# Filter by platform
ssmctl list --platform linux
ssmctl list --platform windows

# JSON output
ssmctl list --output json
```

Requires `ssm:DescribeInstanceInformation` and `ec2:DescribeInstances` permissions.

---

### Connect to an instance

```bash
ssmctl connect <target>
```

Starts an interactive SSM session. Requires the [AWS Session Manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html).

---

### Forward a port

```bash
ssmctl forward <target> --local <local-port> --remote <remote-port-or-host:port>
```

Tunnels a local port to a port on the instance (or to a host reachable from the instance) through SSM Session Manager. The command blocks until interrupted with Ctrl-C, which cleanly terminates the session.

```bash
# Forward local :5432 to the instance's own localhost:5432
ssmctl forward web-1 --local 5432 --remote 5432

# Forward local :5432 to an RDS endpoint reachable from the instance
ssmctl forward web-1 --local 5432 --remote rds.internal.example.com:5432

# Use a different local port
ssmctl forward web-1 --local 15432 --remote rds.internal.example.com:5432
```

`--remote` is interpreted automatically:
- bare integer (e.g. `5432`) — uses `AWS-StartPortForwardingSession` (local-only)
- `host:port` — uses `AWS-StartPortForwardingSessionToRemoteHost`

Requires the [AWS Session Manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) and `ssm:StartSession` permission on the target instance.

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

### Large file transfers via S3

The default `cp` path is bounded by SSM Run Command payload limits (~2 MB
uploads, ~36 KB downloads). For larger files, stage the transfer through an
S3 bucket with `--via s3://<bucket>[/<prefix>]`:

```bash
# Upload — local → S3 → instance
ssmctl cp --via s3://my-bucket/tmp ./large-file.tar.gz web-1:/var/data/large-file.tar.gz

# Download — instance → S3 → local
ssmctl cp --via s3://my-bucket/tmp web-1:/var/log/big.log ./big.log

# Keep the S3 staging object after a successful transfer (useful for debugging)
ssmctl cp --via s3://my-bucket/tmp --keep-staging ./file.bin web-1:/tmp/file.bin
```

How it works:

- **Upload:** the local file is uploaded to a unique staging key under the
  prefix you provided, then `aws s3 cp` runs on the target via SSM to pull
  the object onto the instance.
- **Download:** `aws s3 cp` runs on the target via SSM to push the file to
  S3, then `ssmctl` downloads the staged object locally.
- The staging object is deleted after a successful transfer unless
  `--keep-staging` is set. Failed transfers also leave the staging object
  in place to aid debugging.

Requirements for the S3-backed path:

- The target instance must have the AWS CLI installed (`aws` on `PATH`).
- The instance role must allow `s3:GetObject` (uploads) and/or `s3:PutObject`
  (downloads) on the staging bucket and prefix.
- The local AWS credentials used by `ssmctl` must allow `s3:PutObject`,
  `s3:GetObject`, and `s3:DeleteObject` on the staging bucket and prefix.

Example IAM policy fragment (tighten to your actual prefix):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject"
      ],
      "Resource": "arn:aws:s3:::my-bucket/tmp/ssmctl-*"
    }
  ]
}
```

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
- For `connect` and `forward`, the [Session Manager plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html) must be installed locally

---

## Target OS support

| Command | Linux/macOS targets | Windows targets |
|---------|---------------------|-----------------|
| `connect` | Supported | Supported when the Session Manager plugin is installed locally |
| `forward` | Supported | Supported when the Session Manager plugin is installed locally |
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
- [x] `ssmctl list` — instance discovery with filtering ([#50](https://github.com/rhysmcneill/ssmctl/issues/50))
- [x] `cp --via s3://...` — lift cp size limits via S3-backed staging ([#13](https://github.com/rhysmcneill/ssmctl/issues/13))
- [x] `ssmctl forward` — port forwarding via Session Manager ([#49](https://github.com/rhysmcneill/ssmctl/issues/49))
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

[![Contributors](https://contrib.rocks/image?repo=rhysmcneill/ssmctl&max=100)](https://github.com/rhysmcneill/ssmctl/graphs/contributors)

---
