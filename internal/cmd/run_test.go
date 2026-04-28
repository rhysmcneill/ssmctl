package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	"github.com/rhysmcneill/ssmctl/internal/config"
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
