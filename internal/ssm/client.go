package ssm

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

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
			return "", err
		}

		if len(instances.Reservations) == 0 || len(instances.Reservations[0].Instances) == 0 {
			return "", fmt.Errorf("instance not found: %s", target)
		}

		return *instances.Reservations[0].Instances[0].InstanceId, nil
	}
}
