package ssm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type mockSSMRunClient struct {
	sendCommandFn          func(ctx context.Context, in *ssm.SendCommandInput, opts ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	getCommandInvocationFn func(ctx context.Context, in *ssm.GetCommandInvocationInput, opts ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
	cancelCommandFn        func(ctx context.Context, in *ssm.CancelCommandInput, opts ...func(*ssm.Options)) (*ssm.CancelCommandOutput, error)
}

func (m *mockSSMRunClient) CancelCommand(ctx context.Context, in *ssm.CancelCommandInput, opts ...func(*ssm.Options)) (*ssm.CancelCommandOutput, error) {
	if m.cancelCommandFn == nil {
		return &ssm.CancelCommandOutput{}, nil
	}
	return m.cancelCommandFn(ctx, in, opts...)
}

func (m *mockSSMRunClient) SendCommand(ctx context.Context, in *ssm.SendCommandInput, opts ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	return m.sendCommandFn(ctx, in, opts...)
}

func (m *mockSSMRunClient) GetCommandInvocation(ctx context.Context, in *ssm.GetCommandInvocationInput, opts ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	return m.getCommandInvocationFn(ctx, in, opts...)
}

func successSendCommand() func(context.Context, *ssm.SendCommandInput, ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	return func(_ context.Context, _ *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
		return &ssm.SendCommandOutput{
			Command: &types.Command{CommandId: aws.String("cmd-123")},
		}, nil
	}
}

func TestRunCommand_Success(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	client := &mockSSMRunClient{
		sendCommandFn: successSendCommand(),
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{
				Status:                types.CommandInvocationStatusSuccess,
				StandardOutputContent: aws.String("hello\n"),
				StandardErrorContent:  aws.String(""),
				ResponseCode:          0,
			}, nil
		},
	}

	result, err := RunCommand(context.Background(), client, "i-123", []string{"echo hello"}, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "hello\n")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestRunCommand_NonZeroExitCode(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	client := &mockSSMRunClient{
		sendCommandFn: successSendCommand(),
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{
				Status:               types.CommandInvocationStatusFailed,
				StandardErrorContent: aws.String("command not found"),
				ResponseCode:         127,
			}, nil
		},
	}

	result, err := RunCommand(context.Background(), client, "i-123", []string{"notacommand"}, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 127 {
		t.Errorf("ExitCode = %d, want 127", result.ExitCode)
	}
}

func TestRunCommand_InvocationDoesNotExistRetry(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	callCount := 0
	client := &mockSSMRunClient{
		sendCommandFn: successSendCommand(),
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			callCount++
			if callCount < 3 {
				return nil, &types.InvocationDoesNotExist{}
			}
			return &ssm.GetCommandInvocationOutput{
				Status:                types.CommandInvocationStatusSuccess,
				StandardOutputContent: aws.String("ok"),
				ResponseCode:          0,
			}, nil
		},
	}

	result, err := RunCommand(context.Background(), client, "i-123", []string{"echo ok"}, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 poll calls, got %d", callCount)
	}
	if result.Stdout != "ok" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "ok")
	}
}

func TestRunCommand_SendCommandError(t *testing.T) {
	client := &mockSSMRunClient{
		sendCommandFn: func(_ context.Context, _ *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	_, err := RunCommand(context.Background(), client, "i-123", []string{"echo hi"}, 30*time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunPowerShellCommand_UsesPowerShellDocument(t *testing.T) {
	var gotDocument string

	client := &mockSSMRunClient{
		sendCommandFn: func(_ context.Context, in *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			gotDocument = aws.ToString(in.DocumentName)
			return &ssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-ps")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			}, nil
		},
	}

	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	if _, err := RunPowerShellCommand(context.Background(), client, "i-win", []string{"Get-Process"}, 30*time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotDocument != runPowerShellDocument {
		t.Fatalf("DocumentName = %q, want %q", gotDocument, runPowerShellDocument)
	}
}

func TestRunCommandForTarget_UsesPlatformDocument(t *testing.T) {
	var gotDocuments []string

	client := &mockSSMRunClient{
		sendCommandFn: func(_ context.Context, in *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
			gotDocuments = append(gotDocuments, aws.ToString(in.DocumentName))
			return &ssm.SendCommandOutput{
				Command: &types.Command{CommandId: aws.String("cmd-target")},
			}, nil
		},
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			return &ssm.GetCommandInvocationOutput{
				Status:       types.CommandInvocationStatusSuccess,
				ResponseCode: 0,
			}, nil
		},
	}

	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	targets := []TargetInfo{
		{InstanceID: "i-linux"},
		{InstanceID: "i-win", Platform: ec2types.PlatformValuesWindows},
	}

	for _, target := range targets {
		if _, err := RunCommandForTarget(context.Background(), client, target, []string{"echo ok"}, 30*time.Second); err != nil {
			t.Fatalf("unexpected error for %s: %v", target.InstanceID, err)
		}
	}

	want := []string{runShellDocument, runPowerShellDocument}
	if len(gotDocuments) != len(want) {
		t.Fatalf("captured %d documents, want %d", len(gotDocuments), len(want))
	}
	for i := range want {
		if gotDocuments[i] != want[i] {
			t.Fatalf("DocumentName[%d] = %q, want %q", i, gotDocuments[i], want[i])
		}
	}
}

func TestRunCommand_ContextCancelled(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	ctx, cancel := context.WithCancel(context.Background())

	client := &mockSSMRunClient{
		sendCommandFn: successSendCommand(),
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			cancel()
			return &ssm.GetCommandInvocationOutput{
				Status: types.CommandInvocationStatusInProgress,
			}, nil
		},
	}

	_, err := RunCommand(ctx, client, "i-123", []string{"sleep 60"}, 30*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestRunCommand_NonSuccessStatuses(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	tests := []struct {
		status  types.CommandInvocationStatus
		details string
		want    string
	}{
		{types.CommandInvocationStatusTimedOut, "ExecutionTimedOut", "command timed out on i-123 (status: TimedOut, details: ExecutionTimedOut)"},
		{statusUndeliverable, "Undeliverable", "command was not delivered to i-123 (status: Undeliverable, details: Undeliverable)"},
		{types.CommandInvocationStatusCancelled, "Cancelled", "command was cancelled on i-123"},
		{statusTerminated, "", "command was terminated on i-123 (status: Terminated)"},
		{types.CommandInvocationStatusFailed, "Failed", "command did not complete on i-123"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			client := &mockSSMRunClient{
				sendCommandFn: successSendCommand(),
				getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
					return &ssm.GetCommandInvocationOutput{
						Status:        tt.status,
						StatusDetails: aws.String(tt.details),
						ResponseCode:  -1,
					}, nil
				},
			}
			res, err := RunCommand(context.Background(), client, "i-123", []string{"x"}, 30*time.Second)
			if res != nil {
				t.Errorf("result = %+v, want nil", res)
			}
			var se *StatusError
			if !errors.As(err, &se) {
				t.Fatalf("err = %v, want *StatusError", err)
			}
			if se.Status != tt.status {
				t.Errorf("Status = %s, want %s", se.Status, tt.status)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func inProgress(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	return &ssm.GetCommandInvocationOutput{Status: types.CommandInvocationStatusInProgress}, nil
}

func TestRunCommand_ClientTimeout(t *testing.T) {
	pollInterval = 5 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	var cancelled *ssm.CancelCommandInput
	client := &mockSSMRunClient{
		sendCommandFn:          successSendCommand(),
		getCommandInvocationFn: inProgress,
		cancelCommandFn: func(_ context.Context, in *ssm.CancelCommandInput, _ ...func(*ssm.Options)) (*ssm.CancelCommandOutput, error) {
			cancelled = in
			return &ssm.CancelCommandOutput{}, nil
		},
	}

	res, err := RunCommand(context.Background(), client, "i-123", []string{"sleep 120"}, 50*time.Millisecond)
	if res != nil {
		t.Errorf("result = %+v, want nil", res)
	}
	var te *TimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want *TimeoutError", err)
	}
	if te.InstanceID != "i-123" || te.Timeout != 50*time.Millisecond {
		t.Errorf("TimeoutError = %+v", te)
	}
	if want := "command timed out after 50ms on i-123"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err.Error(), want)
	}
	if cancelled == nil || aws.ToString(cancelled.CommandId) != "cmd-123" || len(cancelled.InstanceIds) != 1 || cancelled.InstanceIds[0] != "i-123" {
		t.Errorf("CancelCommand input = %+v, want cmd-123 on i-123", cancelled)
	}
}

func TestRunCommand_ClientTimeout_InFlightPollIsTimeout(t *testing.T) {
	pollInterval = 5 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	client := &mockSSMRunClient{
		sendCommandFn: successSendCommand(),
		getCommandInvocationFn: func(ctx context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	_, err := RunCommand(context.Background(), client, "i-123", []string{"x"}, 30*time.Millisecond)
	var te *TimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want *TimeoutError", err)
	}
}

func TestRunCommand_ClientTimeout_CancelErrorIgnored(t *testing.T) {
	pollInterval = 5 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	client := &mockSSMRunClient{
		sendCommandFn:          successSendCommand(),
		getCommandInvocationFn: inProgress,
		cancelCommandFn: func(context.Context, *ssm.CancelCommandInput, ...func(*ssm.Options)) (*ssm.CancelCommandOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	_, err := RunCommand(context.Background(), client, "i-123", []string{"x"}, 30*time.Millisecond)
	var te *TimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want *TimeoutError despite cancel failure", err)
	}
}

func TestRunCommand_ParentCancelIsNotTimeout(t *testing.T) {
	pollInterval = 5 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	ctx, cancel := context.WithCancel(context.Background())
	client := &mockSSMRunClient{
		sendCommandFn: successSendCommand(),
		getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
			cancel()
			return &ssm.GetCommandInvocationOutput{Status: types.CommandInvocationStatusInProgress}, nil
		},
	}

	_, err := RunCommand(ctx, client, "i-123", []string{"x"}, time.Minute)
	var te *TimeoutError
	if errors.As(err, &te) {
		t.Errorf("parent cancel reported as TimeoutError: %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestRunCommand_SendsExecutionTimeout(t *testing.T) {
	pollInterval = 5 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	tests := []struct {
		timeout time.Duration
		want    string
	}{
		{90 * time.Second, "90"},
		{1500 * time.Millisecond, "2"},
		{300 * time.Second, "300"},
	}
	for _, tt := range tests {
		t.Run(tt.timeout.String(), func(t *testing.T) {
			var got *ssm.SendCommandInput
			client := &mockSSMRunClient{
				sendCommandFn: func(_ context.Context, in *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
					got = in
					return &ssm.SendCommandOutput{Command: &types.Command{CommandId: aws.String("cmd-123")}}, nil
				},
				getCommandInvocationFn: func(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
					return &ssm.GetCommandInvocationOutput{Status: types.CommandInvocationStatusSuccess}, nil
				},
			}
			if _, err := RunCommand(context.Background(), client, "i-123", []string{"x"}, tt.timeout); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v := got.Parameters["executionTimeout"]; len(v) != 1 || v[0] != tt.want {
				t.Errorf("executionTimeout = %v, want [%s]", v, tt.want)
			}
			if aws.ToInt32(got.TimeoutSeconds) != 30 {
				t.Errorf("TimeoutSeconds = %d, want 30 (SSM minimum delivery timeout)", aws.ToInt32(got.TimeoutSeconds))
			}
			if v := got.Parameters["commands"]; len(v) != 1 || v[0] != "x" {
				t.Errorf("commands = %v", v)
			}
		})
	}
}
