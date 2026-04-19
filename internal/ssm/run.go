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

// SSMRunAPI is the subset of ssm.Client used by RunCommand.
// *ssm.Client satisfies this interface automatically.
type SSMRunAPI interface {
	SendCommand(ctx context.Context, in *ssm.SendCommandInput, opts ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	GetCommandInvocation(ctx context.Context, in *ssm.GetCommandInvocationInput, opts ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
}

// pollInterval controls how often GetCommandInvocation is called.
// Overridden in tests to avoid slow ticker waits.
var pollInterval = 2 * time.Second

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func RunCommand(ctx context.Context, client SSMRunAPI, instanceID string, command []string, timeout time.Duration) (*Result, error) {
	resp, err := client.SendCommand(ctx, &ssm.SendCommandInput{
		InstanceIds:  []string{instanceID},
		DocumentName: aws.String("AWS-RunShellScript"),
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
			return nil, ctx.Err()
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
