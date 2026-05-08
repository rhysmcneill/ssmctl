package ssm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// PortForwardingTarget describes a port forwarding tunnel.
// When RemoteHost is empty the tunnel is local-only (the instance forwards
// to its own localhost). When RemoteHost is set the tunnel is forwarded to
// that remote host as seen from the instance.
type PortForwardingTarget struct {
	LocalPort  int
	RemoteHost string
	RemotePort int
}

// pluginRunner is the function used to invoke the session-manager-plugin
// subprocess. It is a package-level variable so tests can override it to
// capture the call without actually spawning the plugin binary.
var pluginRunner = runSessionManagerPlugin

// ParseRemoteFlag parses the value of the --remote flag.
//
// Accepted forms:
//   - bare integer (e.g. "5432")    -> returns ("", 5432, nil)  local-only
//   - "host:port" (e.g. "db:5432") -> returns ("db", 5432, nil) remote-host
func ParseRemoteFlag(s string) (host string, port int, err error) {
	if s == "" {
		return "", 0, fmt.Errorf("--remote must not be empty")
	}

	// If there is a colon it is the host:port form. Note that IPv6 addresses
	// would need bracket notation, but SSM documents only support hostnames
	// and IPv4 so we keep the simple split.
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		host = s[:idx]
		portStr := s[idx+1:]
		if host == "" {
			return "", 0, fmt.Errorf("--remote %q: host must not be empty when using host:port form", s)
		}
		p, parseErr := parsePort(portStr)
		if parseErr != nil {
			return "", 0, fmt.Errorf("--remote %q: %w", s, parseErr)
		}
		return host, p, nil
	}

	// Bare integer — local-only forwarding.
	p, parseErr := parsePort(s)
	if parseErr != nil {
		return "", 0, fmt.Errorf("--remote %q: %w", s, parseErr)
	}
	return "", p, nil
}

func parsePort(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q: must be an integer", s)
	}
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("invalid port %d: must be between 1 and 65535", n)
	}
	return n, nil
}

// StartPortForwardingSession opens an SSM port-forwarding tunnel. It calls
// the SSM API to start a session then hands control to the local
// session-manager-plugin process, which manages the actual WebSocket tunnel.
//
// The function blocks until the plugin exits (i.e. until the user presses
// Ctrl-C or the session is otherwise terminated). Signal propagation to the
// plugin subprocess is handled automatically through the shared process group.
func StartPortForwardingSession(ctx context.Context, client ClientAPI, instanceID, region, profile string, fwd PortForwardingTarget) error {
	if region == "" {
		return fmt.Errorf("could not determine AWS region: set --region, AWS_REGION, AWS_DEFAULT_REGION, or configure a default region in ~/.aws/config")
	}

	var docName string
	params := map[string][]string{
		"portNumber":      {strconv.Itoa(fwd.RemotePort)},
		"localPortNumber": {strconv.Itoa(fwd.LocalPort)},
	}

	if fwd.RemoteHost == "" {
		docName = "AWS-StartPortForwardingSession"
	} else {
		docName = "AWS-StartPortForwardingSessionToRemoteHost"
		params["host"] = []string{fwd.RemoteHost}
	}

	resp, err := client.StartSession(ctx, &ssm.StartSessionInput{
		Target:       aws.String(instanceID),
		DocumentName: aws.String(docName),
		Parameters:   params,
	})
	if err != nil {
		return fmt.Errorf("failed to start port-forwarding session: %w", err)
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal session response: %w", err)
	}

	inputJSON, err := json.Marshal(map[string]string{"Target": instanceID})
	if err != nil {
		return fmt.Errorf("failed to marshal session input: %w", err)
	}

	return pluginRunner(ctx, string(respJSON), region, profile, string(inputJSON))
}

// runSessionManagerPlugin resolves the session-manager-plugin binary and runs
// it with the standard argument signature used by both connect and forward.
// It is kept as a named function (rather than an anonymous closure) so it can
// be assigned to the pluginRunner var without a cyclic reference.
func runSessionManagerPlugin(ctx context.Context, respJSON, region, profile, inputJSON string) error {
	endpoint := "https://ssm." + region + ".amazonaws.com"

	pluginPath, err := exec.LookPath("session-manager-plugin")
	if err != nil {
		return fmt.Errorf("session-manager-plugin not found in PATH: %w", err)
	}

	cmd := exec.CommandContext(ctx, pluginPath, // #nosec G204 -- pluginPath resolved via exec.LookPath, not user input
		respJSON,
		region,
		"StartSession",
		profile,
		inputJSON,
		endpoint,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("session-manager-plugin exited with error: %w", err)
	}
	return nil
}
