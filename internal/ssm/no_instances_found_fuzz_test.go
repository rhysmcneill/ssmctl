package ssm

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/AdamKorcz/go-118-fuzz-build/testing"
)

func FuzzNoInstancesFound(f *testing.F) {
	f.Add(0) // nil output
	f.Add(1) // empty reservations
	f.Add(2) // single reservation with instances
	f.Add(3) // multiple reservations with instances
	f.Add(4) // reservation with no instances

	f.Fuzz(func(t *testing.T, scenario int) {
		output := generateNoInstancesScenario(scenario)
		result := noInstancesFound(output)

		// Invariant: returns true when output is nil
		if output == nil && !result {
			t.Errorf("noInstancesFound: expected true for nil output, got false")
		}

		// Invariant: returns true when reservations are empty
		if output != nil && len(output.Reservations) == 0 && !result {
			t.Errorf("noInstancesFound: expected true for empty reservations, got false")
		}

		// Invariant: returns false when reservations exist
		if output != nil && len(output.Reservations) > 0 && result {
			t.Errorf("noInstancesFound: expected false for non-empty reservations, got true")
		}
	})
}

func generateNoInstancesScenario(scenario int) *ec2.DescribeInstancesOutput {
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
					Instances: []ec2types.Instance{
						{InstanceId: aws.String("i-0123456789abcdef0")},
					},
				},
			},
		}
	case 3:
		return &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{
					Instances: []ec2types.Instance{
						{InstanceId: aws.String("i-first")},
					},
				},
				{
					Instances: []ec2types.Instance{
						{InstanceId: aws.String("i-second")},
					},
				},
			},
		}
	case 4:
		return &ec2.DescribeInstancesOutput{
			Reservations: []ec2types.Reservation{
				{
					Instances: []ec2types.Instance{},
				},
			},
		}
	default:
		return nil
	}
}
