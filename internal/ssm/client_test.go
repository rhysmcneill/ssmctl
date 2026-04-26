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
	input  *ec2.DescribeInstancesInput
}

func (m *mockEC2Client) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	m.input = in
	return m.output, m.err
}

func TestResolveTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		mockEC2 EC2DescribeInstancesAPI
		wantID  string
		wantErr bool
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

func TestResolveTargetInfo(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		mockEC2     *mockEC2Client
		wantID      string
		wantWindows bool
		wantErr     bool
	}{
		{
			name:   "name lookup keeps Windows platform metadata",
			target: "windows-server",
			mockEC2: &mockEC2Client{output: &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{
				{Instances: []types.Instance{{InstanceId: aws.String("i-windows"), Platform: types.PlatformValuesWindows}}},
			}}},
			wantID:      "i-windows",
			wantWindows: true,
		},
		{
			name:   "instance ID lookup keeps Windows platform metadata",
			target: "i-1234567890abcdef0",
			mockEC2: &mockEC2Client{output: &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{
				{Instances: []types.Instance{{InstanceId: aws.String("i-1234567890abcdef0"), Platform: types.PlatformValuesWindows}}},
			}}},
			wantID:      "i-1234567890abcdef0",
			wantWindows: true,
		},
		{
			name:    "instance ID metadata lookup failure preserves passthrough",
			target:  "i-1234567890abcdef0",
			mockEC2: &mockEC2Client{err: errors.New("access denied")},
			wantID:  "i-1234567890abcdef0",
			wantErr: false,
		},
		{
			name:   "name lookup not found still errors",
			target: "missing-server",
			mockEC2: &mockEC2Client{output: &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{},
			}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ResolveTargetInfo(context.Background(), tt.mockEC2, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveTargetInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
			if info.InstanceID != tt.wantID {
				t.Errorf("ResolveTargetInfo().InstanceID = %q, want %q", info.InstanceID, tt.wantID)
			}
			if info.IsWindows() != tt.wantWindows {
				t.Errorf("ResolveTargetInfo().IsWindows() = %v, want %v", info.IsWindows(), tt.wantWindows)
			}
		})
	}
}
