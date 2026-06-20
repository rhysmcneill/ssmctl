package ssm

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/AdamKorcz/go-118-fuzz-build/testing"
)

func FuzzNameTag(f *testing.F) {
	f.Add(0)  // nil/empty tags
	f.Add(1)  // single tag, no Name key
	f.Add(2)  // single Name tag
	f.Add(3)  // multiple tags, Name in middle
	f.Add(4)  // multiple tags, Name at end
	f.Add(5)  // Name tag with empty value
	f.Add(10) // many tags, no Name

	f.Fuzz(func(t *testing.T, scenario int) {
		tags := generateTagScenario(scenario)
		result := nameTag(tags)

		// Invariant: result is always a string (never nil)
		if result == "" && !hasNameTag(tags) {
			// Empty result is correct if no Name tag exists
			return
		}

		// Invariant: if Name tag exists, result matches its value
		for _, tag := range tags {
			if aws.ToString(tag.Key) == "Name" {
				if result != aws.ToString(tag.Value) {
					t.Errorf("nameTag: expected %q, got %q", aws.ToString(tag.Value), result)
				}
				return
			}
		}

		// If we reach here and result is not empty, something is wrong
		if result != "" {
			t.Errorf("nameTag: expected empty string (no Name tag found), got %q", result)
		}
	})
}

func generateTagScenario(scenario int) []ec2types.Tag {
	switch scenario {
	case 0:
		return nil
	case 1:
		return []ec2types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("prod")},
		}
	case 2:
		return []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String("my-instance")},
		}
	case 3:
		return []ec2types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("prod")},
			{Key: aws.String("Name"), Value: aws.String("web-server")},
			{Key: aws.String("Team"), Value: aws.String("backend")},
		}
	case 4:
		return []ec2types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("prod")},
			{Key: aws.String("Team"), Value: aws.String("backend")},
			{Key: aws.String("Name"), Value: aws.String("db-primary")},
		}
	case 5:
		return []ec2types.Tag{
			{Key: aws.String("Name"), Value: aws.String("")},
		}
	case 10:
		return []ec2types.Tag{
			{Key: aws.String("Owner"), Value: aws.String("alice")},
			{Key: aws.String("Environment"), Value: aws.String("staging")},
			{Key: aws.String("CostCenter"), Value: aws.String("eng")},
			{Key: aws.String("Application"), Value: aws.String("api")},
		}
	default:
		return nil
	}
}

func hasNameTag(tags []ec2types.Tag) bool {
	for _, tag := range tags {
		if aws.ToString(tag.Key) == "Name" {
			return true
		}
	}
	return false
}
