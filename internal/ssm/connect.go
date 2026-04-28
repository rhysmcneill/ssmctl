package ssm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// ClientAPI is the superset of SSM client methods used by this package.
// It combines RunAPI (used by RunCommand, Upload, Download) with StartSession
// so that a single interface can be stored in app.App and mocked in tests.
// *ssm.Client satisfies this interface automatically.
type ClientAPI interface {
	RunAPI
	StartSession(ctx context.Context, params *ssm.StartSessionInput, optFns ...func(*ssm.Options)) (*ssm.StartSessionOutput, error)
}

// StartSession starts an interactive SSM session with a target instance
func StartSession(ctx context.Context, client ClientAPI, instanceID, region, profile string) error {
	if region == "" {
		return fmt.Errorf("could not determine AWS region: set --region, AWS_REGION, AWS_DEFAULT_REGION, or configure a default region in ~/.aws/config")
	}

	resp, err := client.StartSession(ctx, &ssm.StartSessionInput{
		Target: &instanceID,
	})

	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response JSON: %w", err)
	}

	inputJSON, err := json.Marshal(map[string]string{"Target": instanceID})
	if err != nil {
		return fmt.Errorf("failed to marshal input JSON: %w", err)
	}

	endpoint := "https://ssm." + region + ".amazonaws.com"

	pluginPath, err := exec.LookPath("session-manager-plugin")
	if err != nil {
		return fmt.Errorf("session-manager-plugin not found in PATH: %w", err)
	}

	cmd := exec.CommandContext(ctx, pluginPath,
		string(respJSON),
		region,
		"StartSession",
		profile,
		string(inputJSON),
		endpoint,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	return nil
}
