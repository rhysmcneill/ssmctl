# Installation

## Prerequisites

Before installing `ssmctl` you need:

1. **AWS credentials** configured — environment variables, `~/.aws/credentials`, an IAM role attached to your machine, or an SSO session. Any method the AWS SDK recognises will work.
2. **SSM Agent** running on your target EC2 instances. It comes pre-installed on Amazon Linux 2, Amazon Linux 2023, and most AWS-provided AMIs. Check the [SSM Agent documentation](https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent.html) if you're using a custom AMI.
3. **Session Manager plugin** — required only for `connect` and `forward`. See [below](#session-manager-plugin).

---

## Install ssmctl

### Homebrew (macOS / Linux) — recommended

```bash
brew tap rhysmcneill/ssmctl
brew install ssmctl
```

Homebrew handles updates: `brew upgrade ssmctl`.

### Direct binary download

Pre-built binaries are attached to every [GitHub release](https://github.com/rhysmcneill/ssmctl/releases). Download the binary for your platform, make it executable, and place it on your `PATH`.

**Linux (amd64):**

```bash
curl -L https://github.com/rhysmcneill/ssmctl/releases/latest/download/ssmctl-linux-amd64 \
  -o /usr/local/bin/ssmctl
chmod +x /usr/local/bin/ssmctl
```

**Linux (arm64):**

```bash
curl -L https://github.com/rhysmcneill/ssmctl/releases/latest/download/ssmctl-linux-arm64 \
  -o /usr/local/bin/ssmctl
chmod +x /usr/local/bin/ssmctl
```

**macOS (Apple Silicon):**

```bash
curl -L https://github.com/rhysmcneill/ssmctl/releases/latest/download/ssmctl-darwin-arm64 \
  -o /usr/local/bin/ssmctl
chmod +x /usr/local/bin/ssmctl
```

**macOS (Intel):**

```bash
curl -L https://github.com/rhysmcneill/ssmctl/releases/latest/download/ssmctl-darwin-amd64 \
  -o /usr/local/bin/ssmctl
chmod +x /usr/local/bin/ssmctl
```

**Windows (amd64):**

Download `ssmctl-windows-amd64.exe` from the [releases page](https://github.com/rhysmcneill/ssmctl/releases), rename it to `ssmctl.exe`, and add it to a directory on your `PATH`.

#### Verify the checksum

Each release includes a `checksums.txt` file:

```bash
curl -L https://github.com/rhysmcneill/ssmctl/releases/latest/download/checksums.txt -o checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

### Build from source

Requires Go 1.26+.

```bash
git clone https://github.com/rhysmcneill/ssmctl.git
cd ssmctl
make build
# Binary lands at bin/ssmctl
```

Or install directly into `$GOPATH/bin`:

```bash
make install
```

---

## Session Manager plugin

`ssmctl connect` and `ssmctl forward` delegate to the AWS Session Manager plugin binary (`session-manager-plugin`). This is an AWS-provided binary separate from the AWS CLI.

**macOS (Homebrew):**

```bash
brew install --cask session-manager-plugin
```

**macOS (manual):**

```bash
curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/mac/sessionmanager-bundle.zip" \
  -o sessionmanager-bundle.zip
unzip sessionmanager-bundle.zip
sudo ./sessionmanager-bundle/install -i /usr/local/sessionmanagerplugin -b /usr/local/bin/session-manager-plugin
```

**Linux:**

```bash
curl "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb" \
  -o session-manager-plugin.deb
sudo dpkg -i session-manager-plugin.deb
```

For RPM-based distributions and full platform coverage, see the [AWS documentation](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html).

**Verify the plugin is installed:**

```bash
session-manager-plugin --version
```

---

## Verify your installation

```bash
# Check the ssmctl version
ssmctl version

# List instances to confirm AWS connectivity
ssmctl list
```

If `ssmctl list` returns instances, you're ready to go. If it returns an error, check your AWS credentials and that your IAM identity has the required permissions — see [docs/iam.md](iam.md).

---

## AWS credentials

`ssmctl` uses the standard AWS SDK credential chain. The following all work:

| Method | How |
|--------|-----|
| Environment variables | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` |
| Shared credentials file | `~/.aws/credentials` |
| AWS config file | `~/.aws/config` |
| IAM instance role | Automatically used on EC2 |
| AWS SSO | `aws sso login --profile <profile>`, then `ssmctl --profile <profile>` |
| ECS task role / EKS Pod Identity | Automatically used in those environments |

### Using named profiles

```bash
# One-off
ssmctl --profile production list

# Via environment
export AWS_PROFILE=production
ssmctl list
```

### Using a specific region

```bash
ssmctl --region eu-west-1 list

# Or via environment
export AWS_DEFAULT_REGION=eu-west-1
ssmctl list
```

---

## Updating

### Homebrew

```bash
brew upgrade ssmctl
```

### Binary

Download the latest release binary using the same `curl` command from the install step above. The new binary replaces the old one in-place.
