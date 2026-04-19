package ssm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type mockSSMRunClient struct {
	sendCommandFn          func(ctx context.Context, in *ssm.SendCommandInput, opts ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	getCommandInvocationFn func(ctx context.Context, in *ssm.GetCommandInvocationInput, opts ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
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
