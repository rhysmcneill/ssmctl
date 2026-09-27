package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	"github.com/rhysmcneill/ssmctl/internal/config"
	"github.com/rhysmcneill/ssmctl/internal/output"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

// fleetApp builds an App whose list/EC2 clients describe three instances
// (api-1 and api-2 online, api-3 offline) and whose SSM client returns
// per-instance stdout and exit codes.
func fleetApp(t *testing.T, outputFormat string, exitCodes map[string]int32) *app.App {
	t.Helper()
	ssmlib.SetPollInterval(10 * time.Millisecond)
	t.Cleanup(func() { ssmlib.SetPollInterval(2 * time.Second) })

	listClient := singlePageListClient([]types.InstanceInformation{
		{InstanceId: aws.String("i-1"), PlatformType: types.PlatformTypeLinux, PingStatus: types.PingStatusOnline},
		{InstanceId: aws.String("i-2"), PlatformType: types.PlatformTypeLinux, PingStatus: types.PingStatusOnline},
		{InstanceId: aws.String("i-3"), PlatformType: types.PlatformTypeLinux, PingStatus: types.PingStatusConnectionLost},
		{InstanceId: aws.String("i-9"), PlatformType: types.PlatformTypeLinux, PingStatus: types.PingStatusOnline},
	})

	names := map[string]string{"i-1": "api-1", "i-2": "api-2", "i-3": "api-3", "i-9": "db-1"}
	ec2Client := &mockEC2CmdClient{
		fn: func(_ context.Context, _ *awsec2.DescribeInstancesInput, _ ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
			var instances []ec2types.Instance
			for id, name := range names {
				instances = append(instances, ec2types.Instance{
					InstanceId: aws.String(id),
					Tags:       []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String(name)}},
				})
			}
			return &awsec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{Instances: instances}}}, nil
		},
	}

	ssmClient := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, in *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			return &awsssm.SendCommandOutput{Command: &types.Command{CommandId: aws.String("cmd-" + in.InstanceIds[0])}}, nil
		},
		getCommandInvocationFn: func(_ context.Context, in *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			id := aws.ToString(in.InstanceId)
			return &awsssm.GetCommandInvocationOutput{
				Status:                types.CommandInvocationStatusSuccess,
				StandardOutputContent: aws.String("hello from " + id + "\nline two\n"),
				ResponseCode:          exitCodes[id],
			}, nil
		},
	}

	return &app.App{
		Config:     &config.Config{Output: outputFormat, Timeout: 30 * time.Second},
		SSMClient:  ssmClient,
		ListClient: listClient,
		EC2Client:  ec2Client,
		Printer:    &output.Printer{Format: outputFormat},
	}
}

func executeRunCmdCapture(a *app.App, args []string) (stdout, stderr string, err error) {
	var outBuf, errBuf bytes.Buffer
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(runCmd())
	root.SetArgs(args)
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	a.Printer.Out = &outBuf
	err = root.ExecuteContext(context.WithValue(context.Background(), app.ContextKey{}, a))
	return outBuf.String(), errBuf.String(), err
}

func TestRunCmd_Filter_PrefixesOutputPerInstance(t *testing.T) {
	a := fleetApp(t, "text", nil)

	stdout, stderr, err := executeRunCmdCapture(a, []string{"run", "--filter", "api", "--", "echo", "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Instances are printed as they finish, so only per-instance blocks are
	// guaranteed to be contiguous, not their relative order.
	blocks := []string{
		"[api-1] hello from i-1\n[api-1] line two\n",
		"[api-2] hello from i-2\n[api-2] line two\n",
	}
	if stdout != blocks[0]+blocks[1] && stdout != blocks[1]+blocks[0] {
		t.Errorf("stdout =\n%s\nwant both instance blocks, each contiguous", stdout)
	}
	if !strings.Contains(stderr, "running on 2 instance(s)...") {
		t.Errorf("stderr = %q, want start notice", stderr)
	}
	if !strings.Contains(stderr, "skipping 1 matching instance(s) that are not online") {
		t.Errorf("stderr = %q, want offline skip notice", stderr)
	}
	if strings.Contains(stdout, "db-1") {
		t.Errorf("stdout contains non-matching instance db-1: %q", stdout)
	}
}

func TestRunCmd_Filter_AnyFailureReturnsExitCodeError(t *testing.T) {
	a := fleetApp(t, "text", map[string]int32{"i-2": 3})

	_, stderr, err := executeRunCmdCapture(a, []string{"run", "--filter", "api", "--", "false"})

	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitCodeError, got %v (%T)", err, err)
	}
	if exitErr.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", exitErr.ExitCode)
	}
	if exitErr.Error() != "command failed on 1 of 2 instances" {
		t.Errorf("Error() = %q", exitErr.Error())
	}
	if !strings.Contains(stderr, "[api-2] command exited with code 3") {
		t.Errorf("stderr = %q, want per-instance exit code line", stderr)
	}
}

func TestRunCmd_Platform_JSONOutputIsArray(t *testing.T) {
	a := fleetApp(t, "json", map[string]int32{"i-9": 2})

	stdout, stderr, err := executeRunCmdCapture(a, []string{"run", "--platform", "linux", "--", "uptime"})

	if strings.Contains(stderr, "running on") {
		t.Errorf("stderr = %q, JSON mode should not print progress notices", stderr)
	}

	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitCodeError, got %v (%T)", err, err)
	}

	var got []multiRunOutput
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("output is not a JSON array: %v\nraw: %s", err, stdout)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 online linux instances", len(got))
	}
	byID := map[string]multiRunOutput{}
	for _, r := range got {
		byID[r.InstanceID] = r
	}
	if r := byID["i-9"]; r.Name != "db-1" || r.ExitCode != 2 || r.Stdout != "hello from i-9\nline two\n" {
		t.Errorf("i-9 result = %+v", r)
	}

	var raw []map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"instance_id", "name", "stdout", "stderr", "exitCode"} {
		if _, ok := raw[0][key]; !ok {
			t.Errorf("JSON object missing key %q: %v", key, raw[0])
		}
	}
}

func TestRunCmd_Filter_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"target with filter", []string{"run", "web-1", "--filter", "api", "--", "uptime"}, "cannot be combined"},
		{"missing target", []string{"run", "--", "uptime"}, "missing <target>"},
		{"empty filter", []string{"run", "--filter", "", "--", "uptime"}, "--filter must not be empty"},
		{"negative concurrency", []string{"run", "--filter", "api", "--concurrency", "-1", "--", "uptime"}, "--concurrency"},
		{"no matches", []string{"run", "--filter", "nothing-matches", "--", "uptime"}, "no online instances matched"},
		{"only offline matches", []string{"run", "--filter", "api-3", "--", "uptime"}, "no online instances matched"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := fleetApp(t, "text", nil)
			_, _, err := executeRunCmdCapture(a, tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestPrefixLines(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"trailing newline", "a\nb\n", "[x] a\n[x] b\n"},
		{"no trailing newline", "a\nb", "[x] a\n[x] b\n"},
		{"blank line kept", "a\n\nb\n", "[x] a\n[x] \n[x] b\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := prefixLines("x", tc.in); got != tc.want {
				t.Errorf("prefixLines(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
