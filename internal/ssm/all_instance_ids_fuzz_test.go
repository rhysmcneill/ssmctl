package ssm

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func FuzzAllInstanceIDs(f *testing.F) {
	f.Add(0)  // nil input
	f.Add(1)  // one reservation, one instance
	f.Add(2)  // one reservation, two instances
	f.Add(3)  // two reservations
	f.Add(5)  // multiple reservations, multiple instances
	f.Add(10) // many instances

	f.Fuzz(func(t *testing.T, numInstances int) {
		output := generateInstancesOutput(numInstances)
		result := allInstanceIDs(output)

		// Invariant: result length matches instance count
		expectedCount := countInstancesInOutput(output)
		if len(result) != expectedCount {
			t.Errorf("allInstanceIDs: got %d IDs, expected %d", len(result), expectedCount)
		}

		// Invariant: no empty strings in result
		for i, id := range result {
			if id == "" {
				t.Errorf("allInstanceIDs: result[%d] is empty string", i)
			}
		}
	})
}

func generateInstancesOutput(numInstances int) *ec2.DescribeInstancesOutput {
	if numInstances <= 0 {
		return nil
	}

	var instances []types.Instance
	for i := 0; i < numInstances; i++ {
		id := aws.String("i-" + string(rune(97+i%26)))
		instances = append(instances, types.Instance{InstanceId: id})
	}

	numReservations := 1 + (numInstances % 3)
	instancesPerReservation := numInstances / numReservations

	var reservations []types.Reservation
	idx := 0
	for i := 0; i < numReservations && idx < numInstances; i++ {
		end := idx + instancesPerReservation
		if i == numReservations-1 {
			end = numInstances
		}
		reservations = append(reservations, types.Reservation{
			Instances: instances[idx:end],
		})
		idx = end
	}

	return &ec2.DescribeInstancesOutput{
		Reservations: reservations,
	}
}

func countInstancesInOutput(output *ec2.DescribeInstancesOutput) int {
	if output == nil {
		return 0
	}
	count := 0
	for _, r := range output.Reservations {
		count += len(r.Instances)
	}
	return count
}
