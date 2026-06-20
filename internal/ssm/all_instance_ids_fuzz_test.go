package ssm

import (
	"testing"

	_ "github.com/AdamKorcz/go-118-fuzz-build/testing"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func FuzzAllInstanceIDs(f *testing.F) {
	f.Add(1, 1, false) // one reservation, one instance per res, no nil IDs
	f.Add(2, 3, false) // two reservations, three instances each
	f.Add(5, 1, true)  // multiple reservations with some nil IDs
	f.Add(0, 0, false) // edge case: zero reservations

	f.Fuzz(func(t *testing.T, numReservations, instancesPerRes int, includeNilID bool) {
		// Clamp values to reasonable ranges
		if numReservations < 0 || numReservations > 100 {
			numReservations = (numReservations%100 + 100) % 100
		}
		if instancesPerRes < 0 || instancesPerRes > 100 {
			instancesPerRes = (instancesPerRes%100 + 100) % 100
		}

		output := generateInstancesOutput(numReservations, instancesPerRes, includeNilID)
		result := allInstanceIDs(output)

		// Invariant: result length matches non-nil instance count
		expectedCount := countNonNilInstances(output)
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

func generateInstancesOutput(numReservations, instancesPerRes int, includeNilID bool) *ec2.DescribeInstancesOutput {
	if numReservations <= 0 {
		return &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{}}
	}

	var reservations []types.Reservation
	counter := 0
	for i := 0; i < numReservations; i++ {
		var instances []types.Instance
		for j := 0; j < instancesPerRes; j++ {
			var id *string
			if includeNilID && counter%3 == 0 {
				id = nil // Some instances have nil IDs
			} else {
				id = aws.String("i-" + string(rune(97+(counter%26))))
			}
			instances = append(instances, types.Instance{InstanceId: id})
			counter++
		}
		reservations = append(reservations, types.Reservation{Instances: instances})
	}

	return &ec2.DescribeInstancesOutput{Reservations: reservations}
}

func countNonNilInstances(output *ec2.DescribeInstancesOutput) int {
	if output == nil {
		return 0
	}
	count := 0
	for _, r := range output.Reservations {
		for _, inst := range r.Instances {
			if inst.InstanceId != nil {
				count++
			}
		}
	}
	return count
}
