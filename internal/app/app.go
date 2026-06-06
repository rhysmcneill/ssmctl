// Package app provides the main application structure and initialization.
// It contains the App type which holds AWS service clients and configuration,
// and the New function to create and configure a new App instance.
package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/rhysmcneill/ssmctl/internal/config"
	"github.com/rhysmcneill/ssmctl/internal/middleware"
	"github.com/rhysmcneill/ssmctl/internal/output"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

// App struct contains the AWS SSM and EC2 clients, configuration, and output printer for the ssmctl application.
type App struct {
	Config     *config.Config
	SSMClient  ssmlib.ClientAPI
	ListClient ssmlib.ListAPI
	EC2Client  ssmlib.EC2DescribeInstancesAPI
	S3Client   ssmlib.S3API
	Printer    *output.Printer
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

	// If the debug flag is set, log AWS SDK initialisation details.
	if cfg.Debug {

		debugLog := log.New(os.Stderr, "[DEBUG] ", log.LstdFlags)
		debugLog.Println("AWS SDK initialized")
		debugLog.Printf("Profile: %s\n", cfg.Profile)
		debugLog.Printf("Region: %s\n", awsCfg.Region)
		debugLog.Printf("Output: %s\n", cfg.Output)
		debugLog.Printf("Timeout: %v\n", cfg.Timeout)
		awsCfg.HTTPClient = &http.Client{
			Transport: &middleware.RedactingTransport{
				Wrapped: http.DefaultTransport,
				Log:     debugLog,
			},
		}
	}

	// Sync the resolved region back onto cfg so commands that need to pass the
	// region explicitly (e.g. the session-manager-plugin invocation) get the
	// value from AWS_REGION / AWS_DEFAULT_REGION / ~/.aws/config, not just
	// the --region flag.
	if cfg.Region == "" {
		cfg.Region = awsCfg.Region
	}

	ssmClient := ssm.NewFromConfig(awsCfg)

	return &App{
		Config:     cfg,
		SSMClient:  ssmClient,
		ListClient: ssmClient,
		EC2Client:  ec2.NewFromConfig(awsCfg),
		S3Client:   s3.NewFromConfig(awsCfg),
		Printer:    &output.Printer{Format: cfg.Output},
	}, nil
}
