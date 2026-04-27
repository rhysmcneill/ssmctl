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

// TargetInfo contains a resolved EC2 target ID and platform metadata.
type TargetInfo struct {
	InstanceID string
	Platform   types.PlatformValues
}

// IsWindows reports whether the resolved target is a Windows instance.
func (t TargetInfo) IsWindows() bool {
	return t.Platform == types.PlatformValuesWindows
}

// ResolveTarget converts a target identifier to an EC2 instance ID.
// The target can be an instance ID (starting with "i-") or an instance name tag.
// It returns an error if the instance is not found or not in running state.
func ResolveTarget(ctx context.Context, ec2Client EC2DescribeInstancesAPI, target string) (string, error) {
	if strings.HasPrefix(target, "i-") {
		return target, nil
	}

	info, err := lookupTargetByName(ctx, ec2Client, target)
	if err != nil {
		return "", err
	}

	return info.InstanceID, nil
}

// ResolveTargetInfo resolves a target identifier and includes platform metadata.
func ResolveTargetInfo(ctx context.Context, ec2Client EC2DescribeInstancesAPI, target string) (TargetInfo, error) {
	if strings.HasPrefix(target, "i-") {
		return lookupTargetByInstanceID(ctx, ec2Client, target), nil
	}

	return lookupTargetByName(ctx, ec2Client, target)
}

func lookupTargetByInstanceID(ctx context.Context, ec2Client EC2DescribeInstancesAPI, instanceID string) TargetInfo {
	fallback := TargetInfo{InstanceID: instanceID}
	if ec2Client == nil {
		return fallback
	}

	instances, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		// Instance IDs used to pass through without an EC2 lookup. Keep that behavior
		// when platform metadata cannot be read, and let the SSM operation fail normally.
		return fallback
	}

	instance, ok := firstInstance(instances)
	if !ok {
		return fallback
	}

	return targetInfoFromInstance(instanceID, instance)
}

func lookupTargetByName(ctx context.Context, ec2Client EC2DescribeInstancesAPI, target string) (TargetInfo, error) {
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
		return TargetInfo{}, fmt.Errorf("describe instance: %w", err)
	}

	instance, ok := firstInstance(instances)
	if !ok {
		return TargetInfo{}, fmt.Errorf("instance not found: %s", target)
	}

	return targetInfoFromInstance(target, instance), nil
}

func firstInstance(instances *ec2.DescribeInstancesOutput) (types.Instance, bool) {
	if instances == nil || len(instances.Reservations) == 0 || len(instances.Reservations[0].Instances) == 0 {
		return types.Instance{}, false
	}

	return instances.Reservations[0].Instances[0], true
}

func targetInfoFromInstance(fallbackID string, instance types.Instance) TargetInfo {
	instanceID := aws.ToString(instance.InstanceId)
	if instanceID == "" {
		instanceID = fallbackID
	}

	return TargetInfo{InstanceID: instanceID, Platform: instance.Platform}
}
