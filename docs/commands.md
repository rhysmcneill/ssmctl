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

Run a one-shot command on one or more instances and stream its output back.

```bash
ssmctl run <target> -- <command> [args...]
ssmctl run --filter <substring> [--platform <os>] -- <command> [args...]
```

The `--` separator is required. Stdout and stderr are streamed to your terminal. The remote exit code is propagated.

`run` uses `AWS-RunShellScript` for Linux/macOS targets and `AWS-RunPowerShellScript` for Windows targets.

### Running across multiple instances

Use `--filter` and/or `--platform` instead of `<target>` to run the command on every matching instance. Matching works the same way as [`ssmctl list`](#ssmctl-list). Instances that are not `Online` in SSM are skipped, and a notice is printed to stderr.

- The command runs on all matching instances **in parallel**. Use `--concurrency` to cap how many run at once on large fleets.
- Each line of stdout/stderr is prefixed with the instance name, or the instance ID if it has no Name tag.
- Each instance's output is printed as soon as that instance finishes, so faster instances appear first. A `running on N instance(s)...` notice is printed to stderr when the run starts.
- With `--output json`, nothing is printed until every instance has finished. The array is then in the same order as `ssmctl list`.
- If **any** instance fails (non-zero exit code or an SSM error), `ssmctl` exits with code `1`.
- With `--output json`, an array of per-instance results is printed.

### Flags

| Flag | Description |
|------|-------------|
| `--timeout, -t` | Maximum time to wait for the command (default: `60s`). Applies per instance |
| `--output json` | Emit stdout/stderr/exit-code as JSON |
| `--filter <string>` | Run on all online instances whose Name tag or ID contains this substring (case-insensitive) |
| `--platform <os>` | Run on all online instances of this platform: `linux` or `windows` |
| `--concurrency <int>` | Maximum instances to run on in parallel with `--filter`/`--platform` (default: `0`, unlimited) |

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

# Run a PowerShell command on a Windows instance
ssmctl run win-app-1 -- Get-Process

# Check nginx on every instance whose name contains "api"
ssmctl run --filter api -- systemctl status nginx

# Check disk space on all Linux instances, 10 at a time
ssmctl run --platform linux --concurrency 10 -- df -h /

# Per-instance results as a JSON array
ssmctl run --filter api --output json -- uptime
```

Example multi-instance JSON output:

```json
[
  {"instance_id": "i-0abc", "name": "api-1", "stdout": "up 3 days\n", "stderr": "", "exitCode": 0},
  {"instance_id": "i-0def", "name": "api-2", "stdout": "", "stderr": "", "exitCode": 0, "error": "failed to send command: ..."}
]
```

### Required IAM permissions

- `ssm:SendCommand` with `AWS-RunShellScript` and/or `AWS-RunPowerShellScript`
- `ssm:GetCommandInvocation`
- `ssm:DescribeInstanceInformation` (only with `--filter`/`--platform`)

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

Linux/macOS targets use POSIX shell utilities under the hood. Windows targets use PowerShell for in-band transfers. The S3-backed path requires the AWS CLI on the instance.

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

# Upload to a Windows target
ssmctl cp .\appsettings.json win-app-1:C:\Temp\appsettings.json
```

**Windows host — drive-letter paths are supported:**

```powershell
# Upload from a Windows local path
ssmctl cp D:\configs\nginx.conf web-1:/etc/nginx/nginx.conf

# Download to a Windows local path
ssmctl cp web-1:/var/log/app.log C:\Users\Admin\logs\app.log
```

### Required IAM permissions

- `ssm:SendCommand` with `AWS-RunShellScript` and/or `AWS-RunPowerShellScript`
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

## `ssmctl param`

Manage AWS Systems Manager Parameter Store parameters. SecureString values are
decrypted automatically on `get` and `list`.

---

### `ssmctl param get`

Fetch a single parameter's value.

```bash
ssmctl param get <name> [--output json]
```

Text output prints **only** the value — no label — making it pipe-friendly:

```bash
export DB_PASS=$(ssmctl param get /myapp/prod/DB_PASSWORD)
```

#### Examples

```bash
# Print the value to stdout
ssmctl param get /myapp/prod/DB_PASSWORD

# Full parameter object as JSON (name, type, version, ARN, last_modified_date)
ssmctl param get /myapp/prod/DB_PASSWORD --output json
```

#### Required IAM permissions

- `ssm:GetParameter`

---

### `ssmctl param list`

List all parameters whose name begins with a given path prefix.

```bash
ssmctl param list <path> [--recursive] [--output json]
```

#### Output

```
NAME                         TYPE           VERSION   LAST MODIFIED
/myapp/prod/DB_PASSWORD      SecureString   3         2026-07-20 09:12:00
/myapp/prod/FEATURE_FLAG_X   String         1         2026-07-18 14:30:00
/myapp/prod/API_URL          String         2         2026-07-15 11:00:00
```

#### Flags

| Flag | Description |
|------|-------------|
| `--recursive` | Include parameters in all sub-paths |
| `--output json` | Emit a JSON array of parameter objects |

#### Examples

```bash
# List parameters under a path
ssmctl param list /myapp/prod/

# Recursively list everything under /myapp/
ssmctl param list /myapp/ --recursive

# Machine-readable output
ssmctl param list /myapp/prod/ --output json
```

#### Required IAM permissions

- `ssm:GetParametersByPath`

---

### `ssmctl param put`

Create a new parameter or update an existing one.

```bash
ssmctl param put <name> <value> [--type String|StringList|SecureString] [--overwrite]
```

`--type` defaults to `String`. Pass `--overwrite` to update a parameter that already exists
(without it, attempting to overwrite returns an error with a reminder to add the flag).

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | `String` | Parameter type: `String`, `StringList`, or `SecureString` |
| `--overwrite` | `false` | Overwrite the parameter if it already exists |

#### Examples

```bash
# Create a plain string parameter
ssmctl param put /myapp/prod/API_URL "https://api.example.com"

# Create an encrypted SecureString
ssmctl param put /myapp/prod/DB_PASSWORD "supersecret" --type SecureString

# Update an existing parameter
ssmctl param put /myapp/prod/DB_PASSWORD "newpassword" --type SecureString --overwrite

# Capture the new version number
ssmctl param put /myapp/prod/DB_PASSWORD "newpassword" --overwrite --output json
```

#### Required IAM permissions

- `ssm:PutParameter`

---

### `ssmctl param delete`

Permanently delete a parameter.

```bash
ssmctl param delete <name> [--output json]
```

#### Examples

```bash
ssmctl param delete /myapp/prod/OLD_SECRET

# Confirm deletion as JSON
ssmctl param delete /myapp/prod/OLD_SECRET --output json
```

#### Required IAM permissions

- `ssm:DeleteParameter`

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

### Host OS (where ssmctl runs)

`ssmctl` runs natively on **Linux, macOS, and Windows** for all commands. Windows drive-letter paths (e.g. `C:\folder\file.txt`) are handled correctly by `cp`.

`connect` and `forward` require the [Session Manager plugin](installation.md#session-manager-plugin) to be installed locally. AWS provides a Windows installer for this — see the installation guide.

### Target OS (the EC2 instance being managed)

| Command | Linux/macOS targets | Windows targets |
|---------|---------------------|-----------------|
| `list` | Supported | Supported |
| `connect` | Supported | Supported |
| `forward` | Supported | Supported |
| `run` | Supported | Supported |
| `cp` | Supported | Supported |
| `param` | Supported | Supported |
| `version` | Supported | Supported |
| `completion` | Supported | Supported |
