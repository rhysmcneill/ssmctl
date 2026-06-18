package ssm

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func FuzzListInstancesFiltering(f *testing.F) {
	f.Add("", "")                 // no filters
	f.Add("web", "")              // name filter only
	f.Add("", "windows")          // platform filter only
	f.Add("prod", "linux")        // both filters
	f.Add("DB", "WINDOWS")        // case variations
	f.Add("nonexistent", "linux") // name filter matches nothing
	f.Add("app", "unsupported")   // platform filter matches nothing

	f.Fuzz(func(t *testing.T, nameFilter, platformFilter string) {
		items := generateSSMInstances()
		names := generateNameMap()

		result := filterInstances(items, names, nameFilter, platformFilter)

		// Invariant: if name filter is set, all results contain the filter substring
		if nameFilter != "" {
			for _, info := range result {
				if !strings.Contains(strings.ToLower(info.Name), strings.ToLower(nameFilter)) {
					t.Errorf("ListInstances: name %q does not contain filter %q", info.Name, nameFilter)
				}
			}
		}

		// Invariant: if platform filter is set, all results match the platform
		if platformFilter != "" {
			for _, info := range result {
				if !strings.EqualFold(info.Platform, platformFilter) {
					t.Errorf("ListInstances: platform %q does not match filter %q", info.Platform, platformFilter)
				}
			}
		}

		// Invariant: results are a subset of all items
		if len(result) > len(items) {
			t.Errorf("ListInstances: result length %d exceeds items length %d", len(result), len(items))
		}
	})
}

func generateSSMInstances() []ssmtypes.InstanceInformation {
	return []ssmtypes.InstanceInformation{
		{
			InstanceId:   aws.String("i-web-prod-1"),
			PlatformType: ssmtypes.PlatformTypeLinux,
			PingStatus:   ssmtypes.PingStatusOnline,
			AgentVersion: aws.String("3.0.0"),
		},
		{
			InstanceId:   aws.String("i-web-prod-2"),
			PlatformType: ssmtypes.PlatformTypeLinux,
			PingStatus:   ssmtypes.PingStatusOnline,
			AgentVersion: aws.String("3.0.0"),
		},
		{
			InstanceId:   aws.String("i-db-prod-1"),
			PlatformType: ssmtypes.PlatformTypeLinux,
			PingStatus:   ssmtypes.PingStatusOnline,
			AgentVersion: aws.String("3.0.0"),
		},
		{
			InstanceId:   aws.String("i-app-windows-1"),
			PlatformType: ssmtypes.PlatformTypeWindows,
			PingStatus:   ssmtypes.PingStatusOnline,
			AgentVersion: aws.String("3.0.0"),
		},
		{
			InstanceId:   aws.String("i-app-windows-2"),
			PlatformType: ssmtypes.PlatformTypeWindows,
			PingStatus:   ssmtypes.PingStatusOnline,
			AgentVersion: aws.String("3.0.0"),
		},
	}
}

func generateNameMap() map[string]string {
	return map[string]string{
		"i-web-prod-1":    "web-server-prod",
		"i-web-prod-2":    "web-server-staging",
		"i-db-prod-1":     "database-primary",
		"i-app-windows-1": "app-windows-prod",
		"i-app-windows-2": "app-windows-test",
	}
}
