package ssm

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ListAPI abstracts the SSM DescribeInstanceInformation call for testability.
// *ssm.Client satisfies this interface automatically.
type ListAPI interface {
	DescribeInstanceInformation(ctx context.Context, in *ssm.DescribeInstanceInformationInput, opts ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
}

// InstanceInfo holds the combined metadata for a single SSM-managed EC2 instance.
type InstanceInfo struct {
	InstanceID   string `json:"instance_id"`
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	AgentVersion string `json:"agent_version"`
	Status       string `json:"status"`
}

// ListInstances returns SSM-managed EC2 instances, optionally filtered by a
// name substring (filter) or platform type (platform). Results from
// DescribeInstanceInformation are paginated automatically. EC2 Name tags are
// fetched in a single batch call; on EC2 errors the name field is left empty
// and the list is still returned.
func ListInstances(ctx context.Context, ssmClient ListAPI, ec2Client EC2DescribeInstancesAPI, filter, platform string) ([]InstanceInfo, error) {
	items, err := describeAllInstances(ctx, ssmClient)
	if err != nil {
		return nil, err
	}

	names := fetchNames(ctx, ec2Client, items)

	var result []InstanceInfo
	for _, item := range items {
		id := aws.ToString(item.InstanceId)
		name := names[id]
		platformStr := string(item.PlatformType)
		status := string(item.PingStatus)

		if filter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
			continue
		}
		if platform != "" && !strings.EqualFold(platformStr, platform) {
			continue
		}

		result = append(result, InstanceInfo{
			InstanceID:   id,
			Name:         name,
			Platform:     platformStr,
			AgentVersion: aws.ToString(item.AgentVersion),
			Status:       status,
		})
	}

	return result, nil
}

func describeAllInstances(ctx context.Context, ssmClient ListAPI) ([]ssmtypes.InstanceInformation, error) {
	var items []ssmtypes.InstanceInformation
	var nextToken *string

	for {
		resp, err := ssmClient.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("describe instance information: %w", err)
		}

		items = append(items, resp.InstanceInformationList...)

		if resp.NextToken == nil {
			break
		}
		nextToken = resp.NextToken
	}

	return items, nil
}

// fetchNames builds a map from instance ID to EC2 Name tag value using a
// single batch DescribeInstances call. On any EC2 error it returns an empty
// map so that instance listing still succeeds without Name enrichment.
func fetchNames(ctx context.Context, ec2Client EC2DescribeInstancesAPI, items []ssmtypes.InstanceInformation) map[string]string {
	names := make(map[string]string, len(items))
	if ec2Client == nil || len(items) == 0 {
		return names
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.InstanceId != nil {
			ids = append(ids, *item.InstanceId)
		}
	}

	resp, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: ids,
	})
	if err != nil {
		return names
	}

	for _, r := range resp.Reservations {
		for _, inst := range r.Instances {
			id := aws.ToString(inst.InstanceId)
			names[id] = nameTag(inst.Tags)
		}
	}

	return names
}

func nameTag(tags []ec2types.Tag) string {
	for _, t := range tags {
		if aws.ToString(t.Key) == "Name" {
			return aws.ToString(t.Value)
		}
	}
	return ""
}
