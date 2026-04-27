// Package ssm provides utilities for interacting with AWS Systems Manager,
// including session management, remote command execution, and file transfers
// to EC2 instances.
package ssm

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// EC2DescribeInstancesAPI is an interface for querying EC2 instances.
// It abstracts the EC2 API for easier testing.
type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// ResolveTarget converts a target identifier to an EC2 instance ID.
// The target can be an instance ID (starting with "i-") or an instance name tag.
// It returns an error if the instance is not found or not in running state.
func ResolveTarget(ctx context.Context, ec2Client EC2DescribeInstancesAPI, target string) (string, error) {
	switch {
	case strings.HasPrefix(target, "i-"):
		return target, nil
	default:
		instances, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("tag:Name"),
					Values: []string{target},
				},
				{
					Name:   aws.String("instance-state-name"),
					Values: []string{string(types.InstanceStateNameRunning)},
				},
			},
		})

		if err != nil {
			return "", fmt.Errorf("describe instance: %w", err)
		}

		if len(instances.Reservations) == 0 || len(instances.Reservations[0].Instances) == 0 {
			return "", fmt.Errorf("instance not found: %s", target)
		}

		return *instances.Reservations[0].Instances[0].InstanceId, nil
	}
}
