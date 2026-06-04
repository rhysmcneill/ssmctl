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
)

// mockSSMCmdClient implements ssmlib.SSMClientAPI for cmd-layer tests.
type mockSSMCmdClient struct {
	sendCommandFn          func(context.Context, *awsssm.SendCommandInput, ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error)
	getCommandInvocationFn func(context.Context, *awsssm.GetCommandInvocationInput, ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error)
}

func (m *mockSSMCmdClient) SendCommand(ctx context.Context, in *awsssm.SendCommandInput, opts ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
	return m.sendCommandFn(ctx, in, opts...)
}

func (m *mockSSMCmdClient) GetCommandInvocation(ctx context.Context, in *awsssm.GetCommandInvocationInput, opts ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
	return m.getCommandInvocationFn(ctx, in, opts...)
}

func (m *mockSSMCmdClient) StartSession(_ context.Context, _ *awsssm.StartSessionInput, _ ...func(*awsssm.Options)) (*awsssm.StartSessionOutput, error) {
	panic("unexpected call to StartSession in run command test")
}

// executeRunCmd builds a root cobra command, attaches runCmd, and executes it
// with the given args and app injected into the context. Errors from RunE are
// returned directly (SilenceErrors suppresses cobra's own printing).
func executeRunCmd(ctx context.Context, a *app.App, args []string) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(runCmd())
	root.SetArgs(args)
	if err := root.ExecuteContext(context.WithValue(ctx, app.ContextKey{}, a)); err != nil {
		return err //nolint:wrapcheck // cobra unwraps RunE errors; wrapping here would hide *ExitCodeError
	}
	return nil
}

// executeRunCmdWithOutput is like executeRunCmd but captures stdout into buf,
// mirroring the root.go pattern of setting a.Printer.Out = cmd.OutOrStdout().
func executeRunCmdWithOutput(ctx context.Context, a *app.App, args []string, buf *bytes.Buffer) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(runCmd())
	root.SetArgs(args)
	root.SetOut(buf)
	if a.Printer != nil {
		a.Printer.Out = buf
	}
	if err := root.ExecuteContext(context.WithValue(ctx, app.ContextKey{}, a)); err != nil {
		return err //nolint:wrapcheck // cobra unwraps RunE errors; wrapping here would hide *ExitCodeError
	}
	return nil
}

func TestRunCmd_NonZeroExitCodeReturnsExitCodeError(t *testing.T) {
	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, _ *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-test")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusFailed,
				ResponseCode: 127,
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Timeout: 30 * time.Second},
		SSMClient: client,
	}

	err := executeRunCmd(context.Background(), a, []string{"run", "i-123", "--", "echo", "hi"})

	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitCodeError, got %v (%T)", err, err)
	}
	if exitErr.ExitCode != 127 {
		t.Errorf("ExitCode = %d, want 127", exitErr.ExitCode)
	}
}

func TestRunCmd_MissingDashDashReturnsError(t *testing.T) {
	a := &app.App{
		Config: &config.Config{Timeout: 30 * time.Second},
	}

	err := executeRunCmd(context.Background(), a, []string{"run", "i-123"})
	if err == nil {
		t.Fatal("expected error for missing --, got nil")
	}
}

func TestRunCmd_ResolveTargetErrorIsWrapped(t *testing.T) {
	ec2Client := &mockEC2CmdClient{
		fn: func(_ context.Context, _ *awsec2.DescribeInstancesInput, _ ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
			return nil, errors.New("ec2 unavailable")
		},
	}

	a := &app.App{
		Config:    &config.Config{Timeout: 30 * time.Second},
		EC2Client: ec2Client,
	}

	err := executeRunCmd(context.Background(), a, []string{"run", "named-target", "--", "echo", "hi"})
	if err == nil {
		t.Fatal("expected resolve target error, got nil")
	}
	if !strings.Contains(err.Error(), "resolve target") {
		t.Fatalf("error = %q, want wrapped resolve target message", err.Error())
	}
}

func TestExitCodeError_Error(t *testing.T) {
	err := &ExitCodeError{ExitCode: 42}
	want := "command exited with code 42"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestExitCodeError_TypeAssertion(t *testing.T) {
	var err error = &ExitCodeError{ExitCode: 127}
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatal("expected errors.As to match *ExitCodeError")
	}
	if exitErr.ExitCode != 127 {
		t.Errorf("ExitCode = %d, want 127", exitErr.ExitCode)
	}
}

func TestRunCmd_JSONOutput(t *testing.T) {
	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, _ *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-json")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:                types.CommandInvocationStatusSuccess,
				StandardOutputContent: aws.String("hello\n"),
				StandardErrorContent:  aws.String(""),
				ResponseCode:          0,
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Output: "json", Timeout: 30 * time.Second},
		SSMClient: client,
		Printer:   &output.Printer{Format: "json"},
	}

	var buf bytes.Buffer
	err := executeRunCmdWithOutput(context.Background(), a, []string{"run", "i-123", "--", "echo", "hello"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got runOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.Stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", got.Stdout, "hello\n")
	}
	if got.ExitCode != 0 {
		t.Errorf("exitCode = %d, want 0", got.ExitCode)
	}
}

func TestRunCmd_JSONOutput_NonZeroExitCode(t *testing.T) {
	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, _ *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-json-fail")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:               types.CommandInvocationStatusFailed,
				StandardErrorContent: aws.String("not found\n"),
				ResponseCode:         127,
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Output: "json", Timeout: 30 * time.Second},
		SSMClient: client,
		Printer:   &output.Printer{Format: "json"},
	}

	var buf bytes.Buffer
	err := executeRunCmdWithOutput(context.Background(), a, []string{"run", "i-123", "--", "badcmd"}, &buf)

	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *ExitCodeError, got %v (%T)", err, err)
	}
	if exitErr.ExitCode != 127 {
		t.Errorf("ExitCode = %d, want 127", exitErr.ExitCode)
	}

	var got runOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.Stderr != "not found\n" {
		t.Errorf("stderr = %q, want %q", got.Stderr, "not found\n")
	}
	if got.ExitCode != 127 {
		t.Errorf("exitCode = %d, want 127", got.ExitCode)
	}
}

func TestRunCmd_DebugFlagDoesNotBreakExecution(t *testing.T) {
	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, _ *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-test")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:                types.CommandInvocationStatusSuccess,
				ResponseCode:          0,
				StandardOutputContent: aws.String("hello"),
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Timeout: 30 * time.Second, Debug: true},
		SSMClient: client,
	}

	err := executeRunCmd(context.Background(), a, []string{"run", "i-123", "--", "echo", "hi"})
	if err != nil {
		t.Fatalf("expected no error with debug flag, got %v", err)
	}
}

func TestRunCmd_RunCommandErrorIsWrapped(t *testing.T) {
	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, _ *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			return nil, errors.New("send failed")
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			t.Fatal("GetCommandInvocation should not be called when SendCommand fails")
			return nil, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Timeout: 30 * time.Second},
		SSMClient: client,
	}

	err := executeRunCmd(context.Background(), a, []string{"run", "i-123", "--", "echo", "hi"})
	if err == nil {
		t.Fatal("expected run command error, got nil")
	}
	if !strings.Contains(err.Error(), "run command") {
		t.Fatalf("error = %q, want wrapped run command message", err.Error())
	}
}

func TestRunCmd_JoinsCommandArgsIntoSingleShellLine(t *testing.T) {
	var gotCommands []string

	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, in *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			gotCommands = append([]string(nil), in.Parameters["commands"]...)
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-joined")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Timeout: 30 * time.Second},
		SSMClient: client,
	}

	if err := executeRunCmd(context.Background(), a, []string{"run", "i-123", "--", "df", "-h", "/"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"df -h /"}
	if len(gotCommands) != len(want) || gotCommands[0] != want[0] {
		t.Fatalf("commands = %#v, want %#v", gotCommands, want)
	}
}

func TestJoinPowerShellArgs_Empty(t *testing.T) {
	if got := joinPowerShellArgs(nil); got != "" {
		t.Fatalf("joinPowerShellArgs(nil) = %q, want empty string", got)
	}
}

func TestJoinPowerShellArgs_QuotesUnsafeCommandName(t *testing.T) {
	got := joinPowerShellArgs([]string{`C:\Program Files\Tool\tool.exe`, "arg value"})
	want := `& 'C:\Program Files\Tool\tool.exe' 'arg value'`
	if got != want {
		t.Fatalf("joinPowerShellArgs() = %q, want %q", got, want)
	}
}

func TestShellAndPowerShellArg_EmptyString(t *testing.T) {
	if got := shellArg(""); got != "''" {
		t.Fatalf("shellArg(\"\") = %q, want %q", got, "''")
	}
	if got := powerShellArg(""); got != "''" {
		t.Fatalf("powerShellArg(\"\") = %q, want %q", got, "''")
	}
}

func TestPowerShellArg_SafeString(t *testing.T) {
	if got := powerShellArg("Get-Process"); got != "Get-Process" {
		t.Fatalf("powerShellArg() = %q, want %q", got, "Get-Process")
	}
}

func TestPowerShellArg_QuotesAtPrefixedString(t *testing.T) {
	if got := powerShellArg("@hash"); got != "'@hash'" {
		t.Fatalf("powerShellArg() = %q, want %q", got, "'@hash'")
	}
}

func TestPowerShellCommandName_EmptyString(t *testing.T) {
	if got := powerShellCommandName(""); got != "& ''" {
		t.Fatalf("powerShellCommandName(\"\") = %q, want %q", got, "& ''")
	}
}

func TestRunCmd_QuotesGroupedAndEmbeddedQuoteArgs(t *testing.T) {
	var gotCommands []string

	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, in *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			gotCommands = append([]string(nil), in.Parameters["commands"]...)
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-quoted")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Timeout: 30 * time.Second},
		SSMClient: client,
	}

	if err := executeRunCmd(context.Background(), a, []string{"run", "i-123", "--", "printf", "%s", "hello world", "it's"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{`printf %s 'hello world' 'it'\''s'`}
	if len(gotCommands) != len(want) || gotCommands[0] != want[0] {
		t.Fatalf("commands = %#v, want %#v", gotCommands, want)
	}
}

func TestRunCmd_PreservesTildePrefixWithoutQuoting(t *testing.T) {
	var gotCommands []string

	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, in *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			gotCommands = append([]string(nil), in.Parameters["commands"]...)
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-tilde")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Timeout: 30 * time.Second},
		SSMClient: client,
	}

	if err := executeRunCmd(context.Background(), a, []string{"run", "i-123", "--", "cat", "~/logs/app.log"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"cat ~/logs/app.log"}
	if len(gotCommands) != len(want) || gotCommands[0] != want[0] {
		t.Fatalf("commands = %#v, want %#v", gotCommands, want)
	}
}

func TestRunCmd_WindowsTargetUsesPowerShellDocument(t *testing.T) {
	var gotDocument string
	var gotCommands []string

	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, in *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			gotDocument = aws.ToString(in.DocumentName)
			gotCommands = append([]string(nil), in.Parameters["commands"]...)
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-win")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			}, nil
		},
	}

	ec2Client := &mockEC2CmdClient{
		fn: func(_ context.Context, _ *awsec2.DescribeInstancesInput, _ ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
			return &awsec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId: aws.String("i-wintest"),
								Platform:   ec2types.PlatformValuesWindows,
							},
						},
					},
				},
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Timeout: 30 * time.Second},
		SSMClient: client,
		EC2Client: ec2Client,
	}

	err := executeRunCmd(context.Background(), a, []string{"run", "i-wintest", "--", "Write-Output", "hello world", "it's"})
	if err != nil {
		t.Fatalf("expected Windows target to run, got %v", err)
	}
	if gotDocument != "AWS-RunPowerShellScript" {
		t.Fatalf("DocumentName = %q, want %q", gotDocument, "AWS-RunPowerShellScript")
	}
	wantCommands := []string{`Write-Output 'hello world' 'it''s'`}
	if len(gotCommands) != len(wantCommands) || gotCommands[0] != wantCommands[0] {
		t.Fatalf("commands = %#v, want %#v", gotCommands, wantCommands)
	}
}

func TestRunCmd_WindowsTargetQuotesCommandPathWithSpaces(t *testing.T) {
	var gotCommands []string

	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, in *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			gotCommands = append([]string(nil), in.Parameters["commands"]...)
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-win-path")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			}, nil
		},
	}

	ec2Client := &mockEC2CmdClient{
		fn: func(_ context.Context, _ *awsec2.DescribeInstancesInput, _ ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
			return &awsec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId: aws.String("i-wintest"),
								Platform:   ec2types.PlatformValuesWindows,
							},
						},
					},
				},
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Timeout: 30 * time.Second},
		SSMClient: client,
		EC2Client: ec2Client,
	}

	err := executeRunCmd(context.Background(), a, []string{"run", "i-wintest", "--", `C:\Program Files\My Tool\tool.exe`, "arg value"})
	if err != nil {
		t.Fatalf("expected Windows target to run, got %v", err)
	}

	wantCommands := []string{`& 'C:\Program Files\My Tool\tool.exe' 'arg value'`}
	if len(gotCommands) != len(wantCommands) || gotCommands[0] != wantCommands[0] {
		t.Fatalf("commands = %#v, want %#v", gotCommands, wantCommands)
	}
}

func TestRunCmd_TextOutputStderr(t *testing.T) {
	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, _ *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-stderr")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:               types.CommandInvocationStatusSuccess,
				StandardErrorContent: aws.String("warning: something\n"),
				ResponseCode:         0,
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Output: "text", Timeout: 30 * time.Second},
		SSMClient: client,
		Printer:   &output.Printer{Format: "text"},
	}

	var buf bytes.Buffer
	err := executeRunCmdWithOutput(context.Background(), a, []string{"run", "i-123", "--", "mycommand"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCmd_DebugFlagWithFailingCommand(t *testing.T) {
	client := &mockSSMCmdClient{
		sendCommandFn: func(_ context.Context, _ *awsssm.SendCommandInput, _ ...func(*awsssm.Options)) (*awsssm.SendCommandOutput, error) {
			return &awsssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-test")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *awsssm.GetCommandInvocationInput, _ ...func(*awsssm.Options)) (*awsssm.GetCommandInvocationOutput, error) {
			return &awsssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusFailed,
				ResponseCode: 1,
			}, nil
		},
	}

	a := &app.App{
		Config:    &config.Config{Timeout: 30 * time.Second, Debug: true},
		SSMClient: client,
	}

	err := executeRunCmd(context.Background(), a, []string{"run", "i-123", "--", "false"})
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr.ExitCode != 1 {
		t.Fatalf("expected exit code 1 with debug flag, got %v", err)
	}
}
