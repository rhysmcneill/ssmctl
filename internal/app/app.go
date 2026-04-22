package app

import (
	"context"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/rhysmcneill/ssmctl/internal/config"
	"github.com/rhysmcneill/ssmctl/internal/output"
)

type App struct {
	Config    *config.Config
	SSMClient *ssm.Client
	EC2Client *ec2.Client
	Printer   *output.Printer
}

type ContextKey struct{}

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
		return nil, err
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
