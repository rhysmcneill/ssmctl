package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/cobra"

	awsec2 "github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/rhysmcneill/ssmctl/internal/app"
	"github.com/rhysmcneill/ssmctl/internal/config"
)

func executeConnectCmd(ctx context.Context, a *app.App, args []string) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(connectCmd())
	root.SetArgs(args)
	return root.ExecuteContext(context.WithValue(ctx, app.ContextKey{}, a)) //nolint:wrapcheck
}

func TestConnectCmd_NoArgReturnsError(t *testing.T) {
	err := executeConnectCmd(context.Background(), nil, []string{"connect"})
	if err == nil {
		t.Fatal("expected error for missing target arg, got nil")
	}
}

func TestConnectCmd_TooManyArgsReturnsError(t *testing.T) {
	err := executeConnectCmd(context.Background(), nil, []string{"connect", "i-123", "i-456"})
	if err == nil {
		t.Fatal("expected error for too many args, got nil")
	}
}

// TestConnectCmd_ResolveTargetError covers the "resolve target: %w" error
// branch in connectCmd by using a Name-style target (not i-xxx) with an EC2
// client that returns an error.
func TestConnectCmd_ResolveTargetError(t *testing.T) {
	ec2Client := &mockEC2CmdClient{
		fn: func(_ context.Context, _ *awsec2.DescribeInstancesInput, _ ...func(*awsec2.Options)) (*awsec2.DescribeInstancesOutput, error) {
			return nil, errors.New("ec2: describe instances failed")
		},
	}

	a := &app.App{
		Config:    &config.Config{Region: "us-east-1", Timeout: 30 * time.Second},
		EC2Client: ec2Client,
	}

	// "my-server" is not an i-xxx prefix so ResolveTarget calls EC2, which fails.
	err := executeConnectCmd(context.Background(), a, []string{"connect", "my-server"})
	if err == nil {
		t.Fatal("expected error from EC2 lookup, got nil")
	}
}
