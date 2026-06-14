package ssm

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func FuzzTargetInfoFromInstance(f *testing.F) {
	f.Add("", "")                               // both empty
	f.Add("fallback-id", "")                    // fallback used, instance ID empty
	f.Add("", "i-1234567890abcdef0")            // no fallback needed
	f.Add("fallback-id", "i-1234567890abcdef0") // both provided
	f.Add("i-fallback", "i-instance")           // both are instance IDs
	f.Add("my-instance", "i-prod-db")           // mixed formats

	f.Fuzz(func(t *testing.T, fallbackID, instanceID string) {
		instance := ec2types.Instance{
			InstanceId: aws.String(instanceID),
			Platform:   ec2types.PlatformValuesWindows,
		}

		result := targetInfoFromInstance(fallbackID, instance)

		// Invariant: if instance ID is non-empty, it takes precedence
		if instanceID != "" && result.InstanceID != instanceID {
			t.Errorf("targetInfoFromInstance: expected %q, got %q (fallback=%q)", instanceID, result.InstanceID, fallbackID)
		}

		// Invariant: if instance ID is empty, fallback is used
		if instanceID == "" && result.InstanceID != fallbackID {
			t.Errorf("targetInfoFromInstance: expected fallback %q, got %q", fallbackID, result.InstanceID)
		}

		// Invariant: platform is preserved from instance
		if result.Platform != ec2types.PlatformValuesWindows {
			t.Errorf("targetInfoFromInstance: expected platform %v, got %v", ec2types.PlatformValuesWindows, result.Platform)
		}
	})
}
