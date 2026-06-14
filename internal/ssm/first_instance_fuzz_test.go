package ssm

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func FuzzFirstInstance(f *testing.F) {
	f.Add(0) // nil output
	f.Add(1) // empty reservations
	f.Add(2) // reservation with no instances
	f.Add(3) // single instance
	f.Add(4) // multiple instances (only first returned)
	f.Add(5) // multiple reservations

	f.Fuzz(func(t *testing.T, scenario int) {
		output := generateInstanceScenario(scenario)
		instance, ok := firstInstance(output)

		// Invariant: ok=false when output is nil or empty
		if output == nil || len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
			if ok {
				t.Errorf("firstInstance: expected ok=false for empty/nil output, got ok=true")
			}
			return
		}

		// Invariant: ok=true when instances exist
		if !ok {
			t.Errorf("firstInstance: expected ok=true for non-empty output, got ok=false")
		}

		// Invariant: returned instance is the first one from first reservation
		expectedID := aws.ToString(output.Reservations[0].Instances[0].InstanceId)
		actualID := aws.ToString(instance.InstanceId)
		if actualID != expectedID {
			t.Errorf("firstInstance: expected instance ID %q, got %q", expectedID, actualID)
		}
	})
}

func generateInstanceScenario(scenario int) *ec2.DescribeInstancesOutput {
	switch scenario {
	case 0:
		return nil
	case 1:
		return &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{},
		}
	case 2:
		return &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{
					Instances: []ec2types.Instance{},
				},
			},
		}
	case 3:
		return &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{
					Instances: []ec2types.Instance{
						{InstanceId: aws.String("i-0123456789abcdef0")},
					},
				},
			},
		}
	case 4:
		return &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{
					Instances: []ec2types.Instance{
						{InstanceId: aws.String("i-first")},
						{InstanceId: aws.String("i-second")},
						{InstanceId: aws.String("i-third")},
					},
				},
			},
		}
	case 5:
		return &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{
					Instances: []ec2types.Instance{
						{InstanceId: aws.String("i-res1-inst1")},
					},
				},
				{
					Instances: []ec2types.Instance{
						{InstanceId: aws.String("i-res2-inst1")},
					},
				},
			},
		}
	default:
		return nil
	}
}
