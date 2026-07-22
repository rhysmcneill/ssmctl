package ssm

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type mockListAPI struct {
	pages [][]ssmtypes.InstanceInformation
	err   error
	calls int
}

func (m *mockListAPI) DescribeInstanceInformation(_ context.Context, _ *ssm.DescribeInstanceInformationInput, _ ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	if m.err != nil {
		return nil, m.err
	}

	page := m.calls
	m.calls++

	out := &ssm.DescribeInstanceInformationOutput{
		InstanceInformationList: m.pages[page],
	}
	if page+1 < len(m.pages) {
		out.NextToken = aws.String("token")
	}

	return out, nil
}

func linuxInstance(id, agent string) ssmtypes.InstanceInformation {
	return ssmtypes.InstanceInformation{
		InstanceId:   aws.String(id),
		PlatformType: ssmtypes.PlatformTypeLinux,
		AgentVersion: aws.String(agent),
		PingStatus:   ssmtypes.PingStatusOnline,
	}
}

func windowsInstance(id, agent string) ssmtypes.InstanceInformation {
	return ssmtypes.InstanceInformation{
		InstanceId:   aws.String(id),
		PlatformType: ssmtypes.PlatformTypeWindows,
		AgentVersion: aws.String(agent),
		PingStatus:   ssmtypes.PingStatusOnline,
	}
}

func ec2WithNames(pairs map[string]string) *mockEC2Client {
	var reservations []ec2types.Reservation
	for id, name := range pairs {
		reservations = append(reservations, ec2types.Reservation{
			Instances: []ec2types.Instance{
				{
					InstanceId: aws.String(id),
					Tags: []ec2types.Tag{
						{Key: aws.String("Name"), Value: aws.String(name)},
					},
				},
			},
		})
	}
	return &mockEC2Client{output: &ec2.DescribeInstancesOutput{Reservations: reservations}}
}

func TestListInstances(t *testing.T) {
	tests := []struct {
		name     string
		ssm      *mockListAPI
		ec2      EC2DescribeInstancesAPI
		filter   string
		platform string
		wantIDs  []string
		wantErr  bool
	}{
		{
			name: "returns all instances when no filters set",
			ssm: &mockListAPI{pages: [][]ssmtypes.InstanceInformation{{
				linuxInstance("i-aaa", "3.2.0"),
				windowsInstance("i-bbb", "3.2.0"),
			}}},
			ec2:     ec2WithNames(map[string]string{"i-aaa": "web-1", "i-bbb": "win-1"}),
			wantIDs: []string{"i-aaa", "i-bbb"},
		},
		{
			name: "filter narrows by name substring case-insensitively",
			ssm: &mockListAPI{pages: [][]ssmtypes.InstanceInformation{{
				linuxInstance("i-aaa", "3.2.0"),
				linuxInstance("i-bbb", "3.2.0"),
			}}},
			ec2:     ec2WithNames(map[string]string{"i-aaa": "web-server", "i-bbb": "bastion"}),
			filter:  "WEB",
			wantIDs: []string{"i-aaa"},
		},
		{
			name: "filter narrows by instance ID substring case-insensitively",
			ssm: &mockListAPI{pages: [][]ssmtypes.InstanceInformation{{
				linuxInstance("i-abc123", "3.2.0"),
				linuxInstance("i-def456", "3.2.0"),
			}}},
			ec2:     ec2WithNames(map[string]string{"i-abc123": "api-server", "i-def456": "worker"}),
			filter:  "ABC",
			wantIDs: []string{"i-abc123"},
		},
		{
			name: "platform filter narrows by platform case-insensitively",
			ssm: &mockListAPI{pages: [][]ssmtypes.InstanceInformation{{
				linuxInstance("i-linux", "3.2.0"),
				windowsInstance("i-win", "3.2.0"),
			}}},
			ec2:      ec2WithNames(map[string]string{"i-linux": "web-1", "i-win": "win-1"}),
			platform: "linux",
			wantIDs:  []string{"i-linux"},
		},
		{
			name: "filter and platform applied together",
			ssm: &mockListAPI{pages: [][]ssmtypes.InstanceInformation{{
				linuxInstance("i-web", "3.2.0"),
				linuxInstance("i-app", "3.2.0"),
				windowsInstance("i-win", "3.2.0"),
			}}},
			ec2:      ec2WithNames(map[string]string{"i-web": "web-server", "i-app": "app-server", "i-win": "win-server"}),
			filter:   "server",
			platform: "linux",
			wantIDs:  []string{"i-web", "i-app"},
		},
		{
			name: "pagination fetches all pages",
			ssm: &mockListAPI{pages: [][]ssmtypes.InstanceInformation{
				{linuxInstance("i-page1", "3.2.0")},
				{linuxInstance("i-page2", "3.2.0")},
			}},
			ec2:     ec2WithNames(map[string]string{"i-page1": "srv-1", "i-page2": "srv-2"}),
			wantIDs: []string{"i-page1", "i-page2"},
		},
		{
			name: "EC2 enrichment failure degrades gracefully with empty names",
			ssm: &mockListAPI{pages: [][]ssmtypes.InstanceInformation{{
				linuxInstance("i-aaa", "3.2.0"),
			}}},
			ec2:     &mockEC2Client{err: errors.New("access denied")},
			wantIDs: []string{"i-aaa"},
		},
		{
			name:    "SSM API error is propagated",
			ssm:     &mockListAPI{err: errors.New("SSM unavailable"), pages: [][]ssmtypes.InstanceInformation{{}}},
			ec2:     &mockEC2Client{},
			wantErr: true,
		},
		{
			name: "nil EC2 client leaves names empty",
			ssm: &mockListAPI{pages: [][]ssmtypes.InstanceInformation{{
				linuxInstance("i-aaa", "3.2.0"),
			}}},
			ec2:     nil,
			wantIDs: []string{"i-aaa"},
		},
		{
			name: "no results when filter matches nothing",
			ssm: &mockListAPI{pages: [][]ssmtypes.InstanceInformation{{
				linuxInstance("i-aaa", "3.2.0"),
			}}},
			ec2:     ec2WithNames(map[string]string{"i-aaa": "bastion"}),
			filter:  "nonexistent",
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ListInstances(context.Background(), tt.ssm, tt.ec2, tt.filter, tt.platform)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListInstances() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if len(got) != len(tt.wantIDs) {
				t.Fatalf("ListInstances() returned %d results, want %d: %v", len(got), len(tt.wantIDs), got)
			}

			gotIDs := make(map[string]bool, len(got))
			for _, info := range got {
				gotIDs[info.InstanceID] = true
			}
			for _, id := range tt.wantIDs {
				if !gotIDs[id] {
					t.Errorf("ListInstances() missing expected instance %q; got %v", id, got)
				}
			}
		})
	}
}

func TestListInstances_NameEnrichment(t *testing.T) {
	ssm := &mockListAPI{pages: [][]ssmtypes.InstanceInformation{{
		linuxInstance("i-aaa", "3.2.0"),
	}}}
	ec2 := ec2WithNames(map[string]string{"i-aaa": "my-server"})

	got, err := ListInstances(context.Background(), ssm, ec2, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Name != "my-server" {
		t.Errorf("Name = %q, want %q", got[0].Name, "my-server")
	}
	if got[0].Platform != "Linux" {
		t.Errorf("Platform = %q, want %q", got[0].Platform, "Linux")
	}
	if got[0].Status != "Online" {
		t.Errorf("Status = %q, want %q", got[0].Status, "Online")
	}
}
