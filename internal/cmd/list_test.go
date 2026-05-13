package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	"github.com/rhysmcneill/ssmctl/internal/config"
	"github.com/rhysmcneill/ssmctl/internal/output"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

// mockListCmdClient is a minimal ListAPI implementation for cmd-layer tests.
type mockListCmdClient struct {
	fn func(context.Context, *awsssm.DescribeInstanceInformationInput, ...func(*awsssm.Options)) (*awsssm.DescribeInstanceInformationOutput, error)
}

func (m *mockListCmdClient) DescribeInstanceInformation(ctx context.Context, in *awsssm.DescribeInstanceInformationInput, opts ...func(*awsssm.Options)) (*awsssm.DescribeInstanceInformationOutput, error) {
	return m.fn(ctx, in, opts...)
}

// mockEC2CmdClient is a minimal EC2DescribeInstancesAPI implementation used by list tests.
type mockEC2CmdClient struct {
	fn func(context.Context, *awsec2.DescribeInstancesInput, ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error)
}

func (m *mockEC2CmdClient) DescribeInstances(ctx context.Context, in *awsec2.DescribeInstancesInput, opts ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
	return m.fn(ctx, in, opts...)
}

// singlePageListClient returns a ListAPI mock that yields one page of instances and then stops.
func singlePageListClient(instances []ssmtypes.InstanceInformation) *mockListCmdClient {
	return &mockListCmdClient{
		fn: func(_ context.Context, _ *awsssm.DescribeInstanceInformationInput, _ ...func(*awsssm.Options)) (*awsssm.DescribeInstanceInformationOutput, error) {
			return &awsssm.DescribeInstanceInformationOutput{
				InstanceInformationList: instances,
			}, nil
		},
	}
}

// noopEC2Client returns an EC2 mock that always returns empty results (name enrichment skipped).
func noopEC2Client() *mockEC2CmdClient {
	return &mockEC2CmdClient{
		fn: func(_ context.Context, _ *awsec2.DescribeInstancesInput, _ ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
			return &awsec2.DescribeInstancesOutput{}, nil
		},
	}
}

func executeListCmdWithOutput(ctx context.Context, a *app.App, args []string, buf *bytes.Buffer) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(listCmd())
	root.SetArgs(args)
	root.SetOut(buf)
	if a.Printer != nil {
		a.Printer.Out = buf
	}
	return root.ExecuteContext(context.WithValue(ctx, app.ContextKey{}, a)) //nolint:wrapcheck
}

// ── printTable unit tests ────────────────────────────────────────────────────

func TestPrintTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := printTable(&buf, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "INSTANCE ID") {
		t.Errorf("expected header line, got: %s", out)
	}
}

func TestPrintTable_WithInstances(t *testing.T) {
	instances := []ssmlib.InstanceInfo{
		{InstanceID: "i-aaa111", Name: "web-1", Platform: "Linux", AgentVersion: "3.1.0", Status: "Online"},
		{InstanceID: "i-bbb222", Name: "db-1", Platform: "Windows", AgentVersion: "3.2.0", Status: "Online"},
	}

	var buf bytes.Buffer
	if err := printTable(&buf, instances); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{"i-aaa111", "web-1", "Linux", "i-bbb222", "db-1", "Windows"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// ── listCmd integration tests ────────────────────────────────────────────────

func TestListCmd_TextOutput(t *testing.T) {
	a := &app.App{
		Config: &config.Config{Output: "text", Timeout: 30 * time.Second},
		ListClient: singlePageListClient([]ssmtypes.InstanceInformation{
			{
				InstanceId:   aws.String("i-abc123"),
				PlatformType: ssmtypes.PlatformTypeLinux,
				PingStatus:   ssmtypes.PingStatusOnline,
				AgentVersion: aws.String("3.1.0"),
			},
		}),
		EC2Client: noopEC2Client(),
		Printer:   &output.Printer{Format: "text"},
	}

	var buf bytes.Buffer
	if err := executeListCmdWithOutput(context.Background(), a, []string{"list"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "i-abc123") {
		t.Errorf("output missing instance ID, got:\n%s", out)
	}
	if !strings.Contains(out, "INSTANCE ID") {
		t.Errorf("output missing table header, got:\n%s", out)
	}
}

func TestListCmd_JSONOutput(t *testing.T) {
	a := &app.App{
		Config: &config.Config{Output: "json", Timeout: 30 * time.Second},
		ListClient: singlePageListClient([]ssmtypes.InstanceInformation{
			{
				InstanceId:   aws.String("i-xyz789"),
				PlatformType: ssmtypes.PlatformTypeLinux,
				PingStatus:   ssmtypes.PingStatusOnline,
				AgentVersion: aws.String("3.1.0"),
			},
		}),
		EC2Client: noopEC2Client(),
		Printer:   &output.Printer{Format: "json"},
	}

	var buf bytes.Buffer
	if err := executeListCmdWithOutput(context.Background(), a, []string{"list"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []ssmlib.InstanceInfo
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(got))
	}
	if got[0].InstanceID != "i-xyz789" {
		t.Errorf("instance_id = %q, want %q", got[0].InstanceID, "i-xyz789")
	}
}

func TestListCmd_EmptyList_TextOutput(t *testing.T) {
	a := &app.App{
		Config:     &config.Config{Output: "text", Timeout: 30 * time.Second},
		ListClient: singlePageListClient(nil),
		EC2Client:  noopEC2Client(),
		Printer:    &output.Printer{Format: "text"},
	}

	var buf bytes.Buffer
	if err := executeListCmdWithOutput(context.Background(), a, []string{"list"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "INSTANCE ID") {
		t.Errorf("expected header even for empty list, got:\n%s", out)
	}
}
