package ssm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// StartSession starts an interactive SSM session with a target instance
func StartSession(ctx context.Context, client *ssm.Client, instanceID, region, profile string) error {
	if region == "" {
		return fmt.Errorf("region is required for SSM session (set --region or AWS_DEFAULT_REGION)")
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
