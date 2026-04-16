# ssmctl Implementation Plan

## v1 Scope

### Commands

```bash
ssmctl connect <target>
ssmctl run <target> -- uname -a
ssmctl cp ./file.txt <target>:/tmp/file.txt
ssmctl cp <target>:/var/log/app.log ./app.log
ssmctl version
```

---

### Global Flags

```bash
--profile, -p   # Use another AWS profile (defaults to AWS_PROFILE)
--region, -r    # Use this region
--output, -o    # Output format: json | text
--debug, -d     # Enable debug logging
--timeout, -t   # Set timeout for a command
```

---

## Package Structure

```text
ssmctl/
├── cmd/ssmctl/main.go
├── internal/
│   ├── cmd/
│   │   ├── root.go
│   │   ├── connect.go
│   │   ├── run.go
│   │   ├── cp.go
│   │   └── version.go
│   ├── app/
│   │   └── app.go
│   ├── config/
│   │   └── config.go
│   ├── ssm/
│   │   ├── client.go
│   │   ├── connect.go
│   │   ├── run.go
│   │   └── transfer.go
│   ├── output/
│   │   └── output.go
│   └── version/
│       └── version.go
├── go.mod
└── go.sum
```

---

## Step 1 — Dependencies

- github.com/spf13/cobra  
- github.com/aws/aws-sdk-go-v2  
- github.com/aws/aws-sdk-go-v2/config  
- github.com/aws/aws-sdk-go-v2/service/ssm  
- github.com/aws/aws-sdk-go-v2/service/ec2  

---

## Step 2 — Config

**internal/config/config.go**

```go
type Config struct {
    Profile string
    Region  string
    Output  string
    Debug   bool
    Timeout time.Duration
}
```

---

## Step 3 — Version

**internal/version/version.go**

```go
var (
    Version   = "dev"
    Commit    = "none"
    BuildDate = "unknown"
)
```

### Makefile flags

```makefile
LDFLAGS = -ldflags "-X github.com/rhysmcneill/ssmctl/internal/version.Version=$(VERSION) \
-X github.com/rhysmcneill/ssmctl/internal/version.Commit=$(COMMIT) \
-X github.com/rhysmcneill/ssmctl/internal/version.BuildDate=$(DATE)"
```

---

## Step 4 — Output Handling

**internal/output/output.go**

```go
type Printer struct {
    Format string
}

func (p *Printer) Print(v any) error
func (p *Printer) Fprintf(w io.Writer, format string, a ...any)
```

- JSON → `json.MarshalIndent`
- Text → `fmt.Println`

---

## Step 5 — App Container

**internal/app/app.go**

```go
type App struct {
    Config    *config.Config
    SSMClient *ssm.Client
    EC2Client *ec2.Client
    Printer   *output.Printer
}
```

```go
func New(cfg *config.Config) (*App, error) {
    // Load AWS config
    // Create SSM + EC2 clients
}
```

---

## Step 6 — Root Command

**internal/cmd/root.go**

```go
func Run() error {
    root := &cobra.Command{ Use: "ssmctl" }

    root.PersistentFlags().StringP("profile", "p", "", "AWS profile")
    root.PersistentFlags().StringP("region", "r", "", "AWS region")
    root.PersistentFlags().StringP("output", "o", "text", "Output format")
    root.PersistentFlags().BoolP("debug", "d", false, "Debug output")
    root.PersistentFlags().DurationP("timeout", "t", 60*time.Second, "Timeout")

    root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
        // Build config
        // Create app
    }

    root.AddCommand(connectCmd(), runCmd(), cpCmd(), versionCmd())

    return root.Execute()
}
```

---

## Step 7 — Target Resolution

**internal/ssm/client.go**

```go
func ResolveTarget(ctx context.Context, client *ssm.Client, target string) (string, error) {
    if strings.HasPrefix(target, "i-") {
        return target, nil
    }

    // Lookup via EC2 / SSM
}
```

---

## Step 8 — Connect

**internal/ssm/connect.go**

```go
func StartSession(ctx context.Context, client *ssm.Client, instanceID string) error {
    resp, err := client.StartSession(ctx, &ssm.StartSessionInput{
        Target: &instanceID,
    })

    // Marshal response
    // Exec session-manager-plugin
}
```

```bash
session-manager-plugin <response_json> <region> StartSession <profile> <input_json> <endpoint_url>
```

---

## Step 9 — Run Command

**internal/ssm/run.go**

```go
func RunCommand(ctx context.Context, client *ssm.Client, instanceID string, command []string, timeout time.Duration) (*Result, error) {
    // SendCommand
    // Poll GetCommandInvocation
}
```

```go
target := args[0]
command := args[cmd.ArgsLenAtDash():]
```

---

## Step 10 — File Transfer

**internal/ssm/transfer.go**

### Argument parsing

```go
func parseArg(s string) (instance, path string, isRemote bool)
```

---

### Upload (local → remote)

- base64 encode file  
- split into ~3KB chunks  
- send via multiple `SendCommand` calls  

```bash
printf '%s' '<chunk>' | base64 -d >> /tmp/._ssmctl_transfer
```

- final step: move to destination  

**Limit:** ~2MB practical

---

### Download (remote → local)

```bash
cat <remote_path> | base64
```

- read stdout  
- decode base64  
- write locally  

**Limit:** ~36KB due to SSM truncation

---

## Step 11 — Makefile

```makefile
VERSION  ?= $(shell git describe --tags --always --dirty)
COMMIT   ?= $(shell git rev-parse --short HEAD)
DATE     ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  = -ldflags "-X .../version.Version=$(VERSION) \
-X .../version.Commit=$(COMMIT) \
-X .../version.BuildDate=$(DATE)"

build:
	go build $(LDFLAGS) -o bin/ssmctl ./cmd/ssmctl

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install $(LDFLAGS) ./cmd/ssmctl

release:
	GOOS=linux  GOARCH=amd64 go build $(LDFLAGS) -o bin/ssmctl-linux-amd64   ./cmd/ssmctl
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/ssmctl-darwin-amd64  ./cmd/ssmctl
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/ssmctl-darwin-arm64  ./cmd/ssmctl
```

---

## Key Gotchas

- `session-manager-plugin` must be installed (`exec.LookPath`)
- `SendCommand` is async → must poll
- stdout is truncated (~48KB)
- `cp` download limited (~36KB usable)
- use `context.WithTimeout` for `--timeout`
- use `ArgsLenAtDash()` for command parsing

---

## Definition of Done (v1)

- connect works via instance ID or Name  
- run executes commands with correct exit code  
- cp upload works for small files  
- cp download works within limits  
- flags behave consistently  
- output supports text + json  
- errors are readable  