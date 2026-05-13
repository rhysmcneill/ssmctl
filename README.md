<div align="center">

<h1><strong>ssmctl</strong></h1>

<p><strong>No bastion hosts. No open ports. No SSH keys.</strong><br>
The ergonomics of <code>ssh</code>, <code>scp</code>, and <code>ssh&nbsp;-L</code> — powered by AWS Systems Manager.</p>

<!-- Replace with the generated GIF once you run: vhs .github/demo.tape -->
<!-- See .github/demo.tape for setup instructions -->

<br>

[![CI](https://github.com/rhysmcneill/ssmctl/actions/workflows/ci.yml/badge.svg)](https://github.com/rhysmcneill/ssmctl/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org)
[![Version](https://img.shields.io/github/v/tag/rhysmcneill/ssmctl)](https://github.com/rhysmcneill/ssmctl/releases)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![Stars](https://img.shields.io/github/stars/rhysmcneill/ssmctl?style=flat)](https://github.com/rhysmcneill/ssmctl)
[![Forks](https://img.shields.io/github/forks/rhysmcneill/ssmctl)](https://github.com/rhysmcneill/ssmctl/forks)

<br>

![Linux](https://img.shields.io/badge/Linux-amd64%20%7C%20arm64-FCC624?logo=linux&logoColor=black)
![macOS](https://img.shields.io/badge/macOS-Intel%20%7C%20Apple%20Silicon-000000?logo=apple)
![Windows](https://img.shields.io/badge/Windows-amd64%20%7C%20arm64-0078D4?logo=windows)

</div>

---

## The problem

You probably access EC2 instances today with something like this:

```bash
aws ssm start-session \
  --target i-0abc1234def5678ab \
  --document-name AWS-StartPortForwardingSessionToRemoteHost \
  --parameters '{"host":["rds.internal"],"portNumber":["5432"],"localPortNumber":["5432"]}'
```

With `ssmctl`, that's:

```bash
ssmctl forward web-1 --local 5432 --remote rds.internal:5432
```

Same AWS APIs. Same security model. Dramatically better interface.

---

## Install

```bash
# Homebrew (macOS / Linux)
brew tap rhysmcneill/ssmctl
brew install ssmctl

# Or grab a binary directly
curl -L https://github.com/rhysmcneill/ssmctl/releases/latest/download/ssmctl-linux-amd64 \
  -o /usr/local/bin/ssmctl && chmod +x /usr/local/bin/ssmctl
```

> Binaries for Linux, macOS, and Windows on every [release](https://github.com/rhysmcneill/ssmctl/releases). See the [installation guide](docs/installation.md) for the Session Manager plugin setup and shell completion instructions.

**Shell completion** — Homebrew users get tab completion automatically. For binary installs, run once:

```bash
# Bash
echo 'source <(ssmctl completion bash)' >> ~/.bashrc

# Zsh
echo 'source <(ssmctl completion zsh)' >> ~/.zshrc

# Fish
ssmctl completion fish > ~/.config/fish/completions/ssmctl.fish
```

---

## Real-world use cases

### Debug a production instance without a bastion

```bash
# Find the instance by Name tag
ssmctl list --filter api

# Drop straight into a shell
ssmctl connect api-server-1
```

No security group changes. No SSH key rotation. No bastion to maintain.

### Connect your local tools to a private RDS database

```bash
ssmctl forward web-1 --local 5432 --remote prod-db.cluster-xyz.eu-west-1.rds.amazonaws.com:5432
```

Then in another terminal, use any Postgres client as if the database were local:

```bash
psql -h localhost -p 5432 -U admin mydb
```

Works equally well for MySQL, Redis, Elasticsearch, Kafka — anything TCP.

### Run a one-off command across an instance without opening a session

```bash
ssmctl run web-1 -- systemctl status nginx
ssmctl run web-1 -- tail -n 100 /var/log/app.log
ssmctl run web-1 -- df -h /
```

Stdout and stderr stream back to your terminal. Exit codes are propagated.

### Pull a log file off an instance with one command

```bash
ssmctl cp web-1:/var/log/app.log ./app.log
```

No SCP. No bastion jump host. For files over ~36 KB, use the S3-backed path:

```bash
ssmctl cp --via s3://my-bucket/staging web-1:/var/log/access.log.2 ./access.log.2
```

---

## Why not just use the AWS CLI?

<div align="center">

| Task | AWS CLI | ssmctl |
|------|---------|--------|
| Interactive shell | `aws ssm start-session --target i-0abc...` | `ssmctl connect web-1` |
| Run a command | `aws ssm send-command --instance-ids ... --document-name AWS-RunShellScript --parameters commands=["uptime"]` | `ssmctl run web-1 -- uptime` |
| Port forward to RDS | 4-line JSON blob | `ssmctl forward web-1 --local 5432 --remote db:5432` |
| Resolve by Name tag | Manual `ec2 describe-instances` first | Built-in |
| Structured output | Manual `--query` / `jq` | `--output json` |

</div>

`ssmctl` wraps the same APIs — it is not a different security model, just a dramatically better interface.

---

## Commands

| Command | Description |
|---------|-------------|
| `ssmctl list` | Discover all SSM-managed instances in your account |
| `ssmctl connect <target>` | Interactive shell session |
| `ssmctl forward <target> --local N --remote host:N` | Port forward to any TCP endpoint |
| `ssmctl run <target> -- <cmd>` | Run a one-shot command and stream output |
| `ssmctl cp ./file <target>:/path` | Upload a file |
| `ssmctl cp <target>:/path ./file` | Download a file |
| `ssmctl cp --via s3://bucket <src> <dst>` | Large file transfer via S3 staging |
| `ssmctl completion [bash\|zsh\|fish\|powershell]` | Generate shell completion scripts |

A `<target>` is an instance ID (`i-0abc...`) or a Name tag (`web-1`). See the [full command reference](docs/commands.md).

---

## Documentation

| | |
|--|--|
| [Command reference](docs/commands.md) | All commands, flags, and examples |
| [Installation guide](docs/installation.md) | Setup, prerequisites, and verification |
| [IAM reference](docs/iam.md) | Exact permissions per command with copy-paste policies |
| [Contributing](CONTRIBUTING.md) | How to build, test, and submit changes |

---

## Contributors

Thanks to everyone who has contributed to ssmctl!

[![Contributors](https://contrib.rocks/image?repo=rhysmcneill/ssmctl&max=100)](https://github.com/rhysmcneill/ssmctl/graphs/contributors)

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) to get started.

---

<div align="center">
MIT License &nbsp;·&nbsp; <a href="https://github.com/rhysmcneill/ssmctl/issues">Report a bug</a> &nbsp;·&nbsp; <a href="https://github.com/rhysmcneill/ssmctl/issues">Request a feature</a>
</div>
