// Package ssm provides utilities for interacting with AWS Systems Manager,
// including session management, remote command execution, and file transfers
// to EC2 instances.
package ssm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// RunAPI is the subset of ssm.Client used by RunCommand.
// *ssm.Client satisfies this interface automatically.
type RunAPI interface {
	SendCommand(ctx context.Context, in *ssm.SendCommandInput, opts ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	GetCommandInvocation(ctx context.Context, in *ssm.GetCommandInvocationInput, opts ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
}

// pollInterval controls how often GetCommandInvocation is called.
// Overridden in tests to avoid slow ticker waits.
var pollInterval = 2 * time.Second

// SetPollInterval overrides the polling interval used by RunCommand.
// Intended for use in tests to avoid slow ticker waits.
func SetPollInterval(d time.Duration) {
	pollInterval = d
}

// Result contains the output and exit code from a remote command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

const (
	runShellDocument      = "AWS-RunShellScript"
	runPowerShellDocument = "AWS-RunPowerShellScript"
)

// RunCommand sends a command to an EC2 instance via Systems Manager and waits
// for completion. It polls for the command invocation status until the command
// finishes or the context is cancelled. Returns the command output and exit code.
func RunCommand(ctx context.Context, client RunAPI, instanceID string, command []string, timeout time.Duration) (*Result, error) {
	return runCommandWithDocument(ctx, client, instanceID, runShellDocument, command, timeout)
}

// RunPowerShellCommand sends a command to a Windows EC2 instance via Systems
// Manager using the AWS-RunPowerShellScript document.
func RunPowerShellCommand(ctx context.Context, client RunAPI, instanceID string, command []string, timeout time.Duration) (*Result, error) {
	return runCommandWithDocument(ctx, client, instanceID, runPowerShellDocument, command, timeout)
}

// RunCommandForTarget chooses the appropriate SSM Run Command document for the
// resolved target platform.
func RunCommandForTarget(ctx context.Context, client RunAPI, target TargetInfo, command []string, timeout time.Duration) (*Result, error) {
	if target.IsWindows() {
		return RunPowerShellCommand(ctx, client, target.InstanceID, command, timeout)
	}
	return RunCommand(ctx, client, target.InstanceID, command, timeout)
}

func runCommandWithDocument(ctx context.Context, client RunAPI, instanceID, documentName string, command []string, timeout time.Duration) (*Result, error) {
	resp, err := client.SendCommand(ctx, &ssm.SendCommandInput{
		InstanceIds:  []string{instanceID},
		DocumentName: aws.String(documentName),
		Parameters: map[string][]string{
			"commands": command,
		},
		TimeoutSeconds: aws.Int32(int32(timeout.Seconds())),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	commandID := aws.ToString(resp.Command.CommandId)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
		case <-ticker.C:
			inv, err := client.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
				CommandId:  aws.String(commandID),
				InstanceId: aws.String(instanceID),
			})
			if err != nil {
				// SSM may not have recorded the invocation yet — retry
				var notFound *types.InvocationDoesNotExist
				if errors.As(err, &notFound) {
					continue
				}
				return nil, fmt.Errorf("failed to get command invocation: %w", err)
			}

			switch inv.Status {
			case types.CommandInvocationStatusPending,
				types.CommandInvocationStatusInProgress,
				types.CommandInvocationStatusDelayed:
				continue
			default:
				return &Result{
					Stdout:   aws.ToString(inv.StandardOutputContent),
					Stderr:   aws.ToString(inv.StandardErrorContent),
					ExitCode: int(inv.ResponseCode),
				}, nil
			}
		}
	}
}
