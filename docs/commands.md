# Command Reference

All commands share the [global flags](#global-flags) defined on the root command.

---

## `ssmctl list`

Discover EC2 instances managed by SSM in your account.

```bash
ssmctl list [--filter <substring>] [--platform linux|windows] [--output json]
```

### Output

```
INSTANCE ID           NAME        PLATFORM   AGENT VERSION   STATUS
i-0123456789abcdef0   web-1       Linux      3.2.2086.0      Online
i-0987654321fedcba0   bastion-1   Linux      3.2.2086.0      Online
i-0aabbccddeeff0011   win-app-1   Windows    3.2.2086.0      Offline
```

### Flags

| Flag | Description |
|------|-------------|
| `--filter <string>` | Substring match on the instance Name tag |
| `--platform linux\|windows` | Filter by platform |
| `--output json` | Emit a JSON array instead of a table |

### Examples

```bash
# All online instances
ssmctl list

# Instances whose Name contains "web"
ssmctl list --filter web

# Linux instances only
ssmctl list --platform linux

# Machine-readable output
ssmctl list --output json | jq '.[].InstanceId'
```

### Required IAM permissions

- `ssm:DescribeInstanceInformation`
- `ec2:DescribeInstances`

---

## `ssmctl connect`

Start an interactive shell session on a target instance.

```bash
ssmctl connect <target>
```

No SSH keys. No open security group rules. The session runs entirely over the SSM WebSocket channel.

### Examples

```bash
ssmctl connect web-1
ssmctl connect i-0123456789abcdef0
```

### Required IAM permissions

- `ssm:StartSession`
- `ssm:TerminateSession` (for clean teardown)

The Session Manager plugin must be [installed locally](installation.md#session-manager-plugin).

---

## `ssmctl forward`

Tunnel a local port to a port on the instance or to a remote host reachable from the instance.

```bash
ssmctl forward <target> --local <port> --remote <port-or-host:port>
```

The command blocks until you press Ctrl-C, which cleanly terminates the SSM session. No port is opened or modified on the target instance — traffic is proxied over the existing SSM tunnel.

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--local <int>` | Yes | Local port to listen on (`1`–`65535`) |
| `--remote <string>` | Yes | Remote port (e.g. `5432`) or `host:port` (e.g. `rds.internal:5432`) |

`--remote` is interpreted automatically:
- **Bare integer** — uses `AWS-StartPortForwardingSession`. Traffic goes to `localhost:<port>` on the instance.
- **`host:port`** — uses `AWS-StartPortForwardingSessionToRemoteHost`. Traffic goes to `<host>:<port>` as seen from the instance.

### Examples

```bash
# Tunnel to a port on the instance itself (e.g. a local Redis)
ssmctl forward web-1 --local 6379 --remote 6379

# Tunnel to an RDS endpoint reachable from the instance
ssmctl forward web-1 --local 5432 --remote prod-db.cluster-xyz.eu-west-1.rds.amazonaws.com:5432

# Use a different local port to avoid conflicts
ssmctl forward web-1 --local 15432 --remote prod-db.cluster-xyz.eu-west-1.rds.amazonaws.com:5432

# Then connect as normal from another terminal
psql -h localhost -p 15432 -U admin mydb
```

### Required IAM permissions

- `ssm:StartSession`
- `ssm:TerminateSession`

The Session Manager plugin must be [installed locally](installation.md#session-manager-plugin).

---

## `ssmctl run`

Run a one-shot command on a target instance and stream its output back.

```bash
ssmctl run <target> -- <command> [args...]
```

The `--` separator is required. Stdout and stderr are streamed to your terminal. The remote exit code is propagated.

> **Note:** `run` targets Linux/macOS instances only. It uses `AWS-RunShellScript` internally. Windows targets require `AWS-RunPowerShellScript`, which is not yet supported.

### Flags

| Flag | Description |
|------|-------------|
| `--timeout, -t` | Maximum time to wait for the command (default: `60s`) |
| `--output json` | Emit stdout/stderr/exit-code as JSON |

### Examples

```bash
# Check disk space
ssmctl run web-1 -- df -h /

# Tail a log file (output is captured at exit, not streamed line-by-line)
ssmctl run web-1 -- tail -n 50 /var/log/app.log

# Longer-running task with an extended timeout
ssmctl run web-1 -t 5m -- /opt/app/migrate.sh

# Capture as JSON for scripting
ssmctl run web-1 --output json -- whoami
```

### Required IAM permissions

- `ssm:SendCommand` with `AWS-RunShellScript`
- `ssm:GetCommandInvocation`

---

## `ssmctl cp`

Copy files to or from a target instance.

```bash
# Upload
ssmctl cp <local-path> <target>:<remote-path>

# Download
ssmctl cp <target>:<remote-path> <local-path>
```

Remote paths use the `<target>:/path` syntax, where `<target>` is an instance ID or Name tag.

> **Note:** `cp` targets Linux/macOS instances only. Transfers use `cat` and `base64` under the hood. Windows targets are not currently supported.

### Size limits (in-band SSM)

| Direction | Limit |
|-----------|-------|
| Upload | ~2 MB (SSM `SendCommand` payload) |
| Download | ~36 KB (SSM `GetCommandInvocation` output) |

For larger files, use the [S3-backed transfer path](#large-files-via-s3).

### Examples

```bash
# Upload a config file
ssmctl cp ./nginx.conf web-1:/etc/nginx/nginx.conf

# Download a log file
ssmctl cp web-1:/var/log/app.log ./app.log
```

### Required IAM permissions

- `ssm:SendCommand` with `AWS-RunShellScript`
- `ssm:GetCommandInvocation`

---

### Large files via S3

For files that exceed the in-band SSM limits, stage the transfer through an S3 bucket:

```bash
ssmctl cp --via s3://<bucket>[/<prefix>] <src> <dst>
```

#### How it works

- **Upload:** the local file is PUT to a unique staging key in S3, then `aws s3 cp` runs on the instance via SSM to pull it down.
- **Download:** `aws s3 cp` runs on the instance to push the file to S3, then `ssmctl` GETs it locally.
- The staging object is **deleted after a successful transfer** by default. Use `--keep-staging` to retain it (useful for debugging).

#### Flags

| Flag | Description |
|------|-------------|
| `--via s3://bucket/prefix` | Enable S3-backed transfer with this staging location |
| `--keep-staging` | Skip deletion of the staging object after transfer |

#### Examples

```bash
# Upload a large archive
ssmctl cp --via s3://my-bucket/ssmctl-staging \
  ./database-dump.tar.gz web-1:/var/backups/database-dump.tar.gz

# Download a large log file
ssmctl cp --via s3://my-bucket/ssmctl-staging \
  web-1:/var/log/access.log.2 ./access.log.2

# Keep the staging object for inspection
ssmctl cp --via s3://my-bucket/ssmctl-staging --keep-staging \
  ./deploy.tar.gz web-1:/opt/app/deploy.tar.gz
```

#### Required IAM permissions

See [docs/iam.md — cp via S3](iam.md#cp-via-s3) for copy-paste policy fragments.

---

## `ssmctl version`

Print the build version, commit SHA, and build date.

```bash
ssmctl version
ssmctl version --output json
```

---

## `ssmctl completion`

Print a shell completion script to stdout. Source it once (or add it to your shell config) to enable tab completion for all ssmctl subcommands and global flags.

```bash
ssmctl completion [bash|zsh|fish|powershell]
```

No AWS credentials are required to run this command.

### Examples

```bash
# Bash — load immediately
source <(ssmctl completion bash)

# Bash — persist across sessions
echo 'source <(ssmctl completion bash)' >> ~/.bashrc

# Zsh — load immediately
source <(ssmctl completion zsh)

# Zsh — persist across sessions
echo 'source <(ssmctl completion zsh)' >> ~/.zshrc

# Fish — load immediately
ssmctl completion fish | source

# Fish — persist across sessions
ssmctl completion fish > ~/.config/fish/completions/ssmctl.fish

# PowerShell — load immediately
ssmctl completion powershell | Out-String | Invoke-Expression
```

See [docs/installation.md — Shell completion](installation.md#shell-completion) for Homebrew and per-shell setup details.

---

## Global flags

These flags apply to every command:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--profile` | `-p` | `$AWS_PROFILE` | AWS named profile |
| `--region` | `-r` | From config/env | AWS region |
| `--output` | `-o` | `text` | Output format: `text` or `json` |
| `--debug` | `-d` | `false` | Enable AWS SDK debug logging |
| `--timeout` | `-t` | `60s` | Command timeout (applies to `run` and `cp`) |

---

## Target resolution

A `<target>` is either:

- An **instance ID** — e.g. `i-0123456789abcdef0`. Passed directly to the AWS API.
- A **Name tag** — e.g. `web-1`. Resolved via an EC2 `DescribeInstances` call filtered by `tag:Name`.

If a Name tag matches more than one running instance, `ssmctl` returns an error and lists the matching IDs so you can be explicit.

---

## Platform support

| Command | Linux/macOS targets | Windows targets |
|---------|---------------------|-----------------|
| `list` | Supported | Supported |
| `connect` | Supported | Supported |
| `forward` | Supported | Supported |
| `run` | Supported | Not supported — requires `AWS-RunPowerShellScript` |
| `cp` | Supported | Not supported — requires POSIX utilities |
| `completion` | Supported | Supported |
