package ssm

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ---------------------------------------------------------------------------
// ParseRemoteFlag
// ---------------------------------------------------------------------------

func TestParseRemoteFlag_BarePort(t *testing.T) {
	host, port, err := ParseRemoteFlag("5432")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "" {
		t.Errorf("host = %q, want empty", host)
	}
	if port != 5432 {
		t.Errorf("port = %d, want 5432", port)
	}
}

func TestParseRemoteFlag_HostPort(t *testing.T) {
	host, port, err := ParseRemoteFlag("db.internal.example.com:5432")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "db.internal.example.com" {
		t.Errorf("host = %q, want %q", host, "db.internal.example.com")
	}
	if port != 5432 {
		t.Errorf("port = %d, want 5432", port)
	}
}

func TestParseRemoteFlag_IPv4HostPort(t *testing.T) {
	host, port, err := ParseRemoteFlag("10.0.0.1:3306")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "10.0.0.1" {
		t.Errorf("host = %q, want %q", host, "10.0.0.1")
	}
	if port != 3306 {
		t.Errorf("port = %d, want 3306", port)
	}
}

func TestParseRemoteFlag_Empty(t *testing.T) {
	_, _, err := ParseRemoteFlag("")
	if err == nil {
		t.Fatal("expected error for empty --remote, got nil")
	}
}

func TestParseRemoteFlag_NonNumericBare(t *testing.T) {
	_, _, err := ParseRemoteFlag("notaport")
	if err == nil {
		t.Fatal("expected error for non-numeric bare value, got nil")
	}
}

func TestParseRemoteFlag_PortTooLow(t *testing.T) {
	_, _, err := ParseRemoteFlag("0")
	if err == nil {
		t.Fatal("expected error for port 0, got nil")
	}
}

func TestParseRemoteFlag_PortTooHigh(t *testing.T) {
	_, _, err := ParseRemoteFlag("65536")
	if err == nil {
		t.Fatal("expected error for port 65536, got nil")
	}
}

func TestParseRemoteFlag_EmptyHostInHostPort(t *testing.T) {
	_, _, err := ParseRemoteFlag(":5432")
	if err == nil {
		t.Fatal("expected error for empty host in host:port form, got nil")
	}
}

func TestParseRemoteFlag_InvalidPortInHostPort(t *testing.T) {
	_, _, err := ParseRemoteFlag("db.example.com:notaport")
	if err == nil {
		t.Fatal("expected error for non-numeric port in host:port form, got nil")
	}
}

func TestParseRemoteFlag_DottedHostname(t *testing.T) {
	host, port, err := ParseRemoteFlag("db.example.com:5432")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "db.example.com" {
		t.Errorf("host = %q, want %q", host, "db.example.com")
	}
	if port != 5432 {
		t.Errorf("port = %d, want 5432", port)
	}
}

// ---------------------------------------------------------------------------
// StartPortForwardingSession
// ---------------------------------------------------------------------------

// capturedInput collects the StartSessionInput received by the mock so tests
// can assert on DocumentName and Parameters without needing the plugin binary.
type capturedStartSession struct {
	input *ssm.StartSessionInput
}

func (c *capturedStartSession) SendCommand(_ context.Context, _ *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	return nil, nil //nolint:nilnil // SendCommand unused in connect/forward tests
}

func (c *capturedStartSession) GetCommandInvocation(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	return nil, nil //nolint:nilnil // GetCommandInvocation unused in connect/forward tests
}

func (c *capturedStartSession) StartSession(_ context.Context, in *ssm.StartSessionInput, _ ...func(*ssm.Options)) (*ssm.StartSessionOutput, error) {
	c.input = in
	return &ssm.StartSessionOutput{
		SessionId:  aws.String("s-test123"),
		StreamUrl:  aws.String("wss://example.com"),
		TokenValue: aws.String("token"),
	}, nil
}

// noopPluginRunner replaces pluginRunner in tests so we never try to exec the
// real session-manager-plugin binary (which is not present in the test env).
func noopPluginRunner(_ context.Context, _, _, _, _ string) error {
	return nil
}

func withNoopPluginRunner(t *testing.T) {
	t.Helper()
	prev := pluginRunner
	pluginRunner = noopPluginRunner
	t.Cleanup(func() { pluginRunner = prev })
}

func TestStartPortForwardingSession_EmptyRegion(t *testing.T) {
	client := &capturedStartSession{}
	err := StartPortForwardingSession(context.Background(), client, "i-123", "", "", PortForwardingTarget{LocalPort: 5432, RemotePort: 5432})
	if err == nil {
		t.Fatal("expected error for empty region, got nil")
	}
}

func TestStartPortForwardingSession_LocalOnly_BuildsCorrectInput(t *testing.T) {
	withNoopPluginRunner(t)
	client := &capturedStartSession{}

	fwd := PortForwardingTarget{LocalPort: 9200, RemotePort: 9200}
	if err := StartPortForwardingSession(context.Background(), client, "i-abc", "eu-west-1", "", fwd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.input == nil {
		t.Fatal("StartSession was not called")
	}
	if got := aws.ToString(client.input.DocumentName); got != "AWS-StartPortForwardingSession" {
		t.Errorf("DocumentName = %q, want %q", got, "AWS-StartPortForwardingSession")
	}

	params := client.input.Parameters
	if v := params["portNumber"]; len(v) != 1 || v[0] != "9200" {
		t.Errorf("portNumber = %v, want [9200]", v)
	}
	if v := params["localPortNumber"]; len(v) != 1 || v[0] != "9200" {
		t.Errorf("localPortNumber = %v, want [9200]", v)
	}
	if _, hasHost := params["host"]; hasHost {
		t.Error("local-only forwarding must not set the 'host' parameter")
	}
}

func TestStartPortForwardingSession_RemoteHost_BuildsCorrectInput(t *testing.T) {
	withNoopPluginRunner(t)
	client := &capturedStartSession{}

	fwd := PortForwardingTarget{LocalPort: 5432, RemoteHost: "rds.internal.example.com", RemotePort: 5432}
	if err := StartPortForwardingSession(context.Background(), client, "i-xyz", "us-east-1", "", fwd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.input == nil {
		t.Fatal("StartSession was not called")
	}
	if got := aws.ToString(client.input.DocumentName); got != "AWS-StartPortForwardingSessionToRemoteHost" {
		t.Errorf("DocumentName = %q, want %q", got, "AWS-StartPortForwardingSessionToRemoteHost")
	}

	params := client.input.Parameters
	if v := params["host"]; len(v) != 1 || v[0] != "rds.internal.example.com" {
		t.Errorf("host = %v, want [rds.internal.example.com]", v)
	}
	if v := params["portNumber"]; len(v) != 1 || v[0] != "5432" {
		t.Errorf("portNumber = %v, want [5432]", v)
	}
	if v := params["localPortNumber"]; len(v) != 1 || v[0] != "5432" {
		t.Errorf("localPortNumber = %v, want [5432]", v)
	}
}

func TestStartPortForwardingSession_DifferentLocalAndRemotePorts(t *testing.T) {
	withNoopPluginRunner(t)
	client := &capturedStartSession{}

	fwd := PortForwardingTarget{LocalPort: 15432, RemotePort: 5432}
	if err := StartPortForwardingSession(context.Background(), client, "i-abc", "eu-west-1", "", fwd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	params := client.input.Parameters
	if v := params["portNumber"]; len(v) != 1 || v[0] != "5432" {
		t.Errorf("portNumber = %v, want [5432]", v)
	}
	if v := params["localPortNumber"]; len(v) != 1 || v[0] != "15432" {
		t.Errorf("localPortNumber = %v, want [15432]", v)
	}
}

func TestStartPortForwardingSession_TargetPassedToAPI(t *testing.T) {
	withNoopPluginRunner(t)
	client := &capturedStartSession{}

	fwd := PortForwardingTarget{LocalPort: 80, RemotePort: 80}
	if err := StartPortForwardingSession(context.Background(), client, "i-target99", "ap-southeast-1", "", fwd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := aws.ToString(client.input.Target); got != "i-target99" {
		t.Errorf("Target = %q, want %q", got, "i-target99")
	}
}

// Ensure the mock satisfies ClientAPI (compile-time check).
var _ ClientAPI = (*capturedStartSession)(nil)

// Ensure unused import of types is avoided — reference it to keep the import
// that future tests may need.
var _ = types.CommandInvocationStatusSuccess
