package ssm

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type mockEC2Client struct {
	output *ec2.DescribeInstancesOutput
	err    error
}

func (m *mockEC2Client) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return m.output, m.err
}

func TestResolveTarget(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		mockEC2   EC2DescribeInstancesAPI
		wantID    string
		wantErr   bool
	}{
		{
			name:    "instance ID passthrough",
			target:  "i-1234567890abcdef0",
			mockEC2: nil,
			wantID:  "i-1234567890abcdef0",
			wantErr: false,
		},
		{
			name:   "name lookup found",
			target: "my-server",
			mockEC2: &mockEC2Client{
				output: &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{
							Instances: []types.Instance{
								{InstanceId: aws.String("i-abcdef1234567890")},
							},
						},
					},
				},
			},
			wantID:  "i-abcdef1234567890",
			wantErr: false,
		},
		{
			name:   "name lookup not found",
			target: "nonexistent-server",
			mockEC2: &mockEC2Client{
				output: &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{},
				},
			},
			wantID:  "",
			wantErr: true,
		},
		{
			name:   "name lookup empty instances in reservation",
			target: "my-server",
			mockEC2: &mockEC2Client{
				output: &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{
						{Instances: []types.Instance{}},
					},
				},
			},
			wantID:  "",
			wantErr: true,
		},
		{
			name:    "name lookup API error",
			target:  "my-server",
			mockEC2: &mockEC2Client{err: errors.New("AWS error")},
			wantID:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := ResolveTarget(context.Background(), tt.mockEC2, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveTarget() error = %v, wantErr %v", err, tt.wantErr)
			}
			if id != tt.wantID {
				t.Errorf("ResolveTarget() = %q, want %q", id, tt.wantID)
			}
		})
	}
}
