// Package app provides the main application structure and initialization.
// It contains the App type which holds AWS service clients and configuration,
// and the New function to create and configure a new App instance.
package app

import (
	"context"
	"fmt"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/rhysmcneill/ssmctl/internal/config"
	"github.com/rhysmcneill/ssmctl/internal/output"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

// App struct contains the AWS SSM and EC2 clients, configuration, and output printer for the ssmctl application.
type App struct {
	Config    *config.Config
	SSMClient ssmlib.ClientAPI
	EC2Client ssmlib.EC2DescribeInstancesAPI
	Printer   *output.Printer
}

// ContextKey is used as a context key type. This is used to store and retrieve values from Go's context.Context.
// The function of this context key is to store and retrieve values across function calls without passing them as parameters.
type ContextKey struct{}

// New creates and initializes a new App with AWS clients configured from the
// provided config. It loads AWS credentials and region from the environment,
// shared config files, or explicit config values.
func New(cfg *config.Config) (*App, error) {
	ctx := context.Background()

	var opts []func(*awscfg.LoadOptions) error

	if cfg.Profile != "" {
		opts = append(opts, awscfg.WithSharedConfigProfile(cfg.Profile))
	}

	if cfg.Region != "" {
		opts = append(opts, awscfg.WithRegion(cfg.Region))
	}

	awsCfg, err := awscfg.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// Sync the resolved region back onto cfg so commands that need to pass the
	// region explicitly (e.g. the session-manager-plugin invocation) get the
	// value from AWS_REGION / AWS_DEFAULT_REGION / ~/.aws/config, not just
	// the --region flag.
	if cfg.Region == "" {
		cfg.Region = awsCfg.Region
	}

	return &App{
		Config:    cfg,
		SSMClient: ssm.NewFromConfig(awsCfg),
		EC2Client: ec2.NewFromConfig(awsCfg),
		Printer:   &output.Printer{Format: cfg.Output},
	}, nil
}
