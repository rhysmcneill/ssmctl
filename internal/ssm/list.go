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

// TagFilter selects instances by EC2 tag. A nil Value matches any instance
// that has the key; otherwise the value must match exactly (case-sensitive).
type TagFilter struct {
	Key   string
	Value *string
}

func (t TagFilter) String() string {
	if t.Value == nil {
		return t.Key
	}
	return t.Key + "=" + *t.Value
}

// ParseTagFilters parses repeatable --tag values of the form "key=value" or
// "key". Only the first "=" separates key from value, so keys such as
// aws:autoscaling:groupName work.
func ParseTagFilters(raw []string) ([]TagFilter, error) {
	filters := make([]TagFilter, 0, len(raw))
	for _, r := range raw {
		key, value, hasValue := strings.Cut(r, "=")
		if key == "" {
			return nil, fmt.Errorf("invalid --tag %q: expected key=value or key", r)
		}
		tf := TagFilter{Key: key}
		if hasValue {
			tf.Value = &value
		}
		filters = append(filters, tf)
	}
	return filters, nil
}

func (t TagFilter) matches(tags map[string]string) bool {
	v, ok := tags[t.Key]
	if !ok {
		return false
	}
	return t.Value == nil || v == *t.Value
}

func describeTagFilters(tags []TagFilter) string {
	parts := make([]string, len(tags))
	for i, t := range tags {
		parts[i] = t.String()
	}
	if len(parts) == 1 {
		return "tag " + parts[0]
	}
	return "tags " + strings.Join(parts, ", ")
}

// instanceMeta holds the EC2 data used to enrich and filter SSM instances.
type instanceMeta struct {
	name string
	tags map[string]string
}

// ListInstances returns SSM-managed EC2 instances, optionally filtered by a
// name substring (filter), platform type (platform) and EC2 tags (tags, ANDed
// together with the other filters). Results from DescribeInstanceInformation
// are paginated automatically. EC2 tags are fetched in a single batch call; on
// EC2 errors the name field is left empty and the list is still returned,
// unless tags were requested, in which case the error is returned. When tags
// are requested and no instance carries them, an error naming the tags is
// returned.
func ListInstances(ctx context.Context, ssmClient ListAPI, ec2Client EC2DescribeInstancesAPI, filter, platform string, tags []TagFilter) ([]InstanceInfo, error) {
	items, err := describeAllInstances(ctx, ssmClient)
	if err != nil {
		return nil, err
	}

	meta, err := fetchInstanceMeta(ctx, ec2Client, items, len(tags) > 0)
	if err != nil {
		return nil, err
	}

	if len(tags) > 0 {
		items = filterByTags(items, meta, tags)
		if len(items) == 0 {
			return nil, fmt.Errorf("no EC2 instances found with %s", describeTagFilters(tags))
		}
	}

	return filterInstances(items, meta, filter, platform), nil
}

func filterByTags(items []ssmtypes.InstanceInformation, meta map[string]instanceMeta, tags []TagFilter) []ssmtypes.InstanceInformation {
	var out []ssmtypes.InstanceInformation
	for _, item := range items {
		instTags := meta[aws.ToString(item.InstanceId)].tags
		matched := true
		for _, t := range tags {
			if !t.matches(instTags) {
				matched = false
				break
			}
		}
		if matched {
			out = append(out, item)
		}
	}
	return out
}

func filterInstances(items []ssmtypes.InstanceInformation, meta map[string]instanceMeta, filter, platform string) []InstanceInfo {
	var result []InstanceInfo
	filterLower := strings.ToLower(filter)
	for _, item := range items {
		id := aws.ToString(item.InstanceId)

		if id == "" {
			continue
		}

		name := meta[id].name
		platformStr := string(item.PlatformType)
		status := string(item.PingStatus)

		if filterLower != "" &&
			!strings.Contains(strings.ToLower(name), filterLower) &&
			!strings.Contains(strings.ToLower(id), filterLower) {
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
	return result

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

// fetchInstanceMeta builds a map from instance ID to its EC2 Name tag and
// full tag set using a single batch DescribeInstances call. On an EC2 error it
// returns an empty map so that listing still succeeds without enrichment,
// unless strict is set (tag filtering needs the data), in which case the error
// is returned.
func fetchInstanceMeta(ctx context.Context, ec2Client EC2DescribeInstancesAPI, items []ssmtypes.InstanceInformation, strict bool) (map[string]instanceMeta, error) {
	meta := make(map[string]instanceMeta, len(items))
	if ec2Client == nil || len(items) == 0 {
		if strict && ec2Client == nil {
			return nil, fmt.Errorf("describe EC2 instances: no EC2 client available for tag filtering")
		}
		return meta, nil
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
		if strict {
			return nil, fmt.Errorf("describe EC2 instances for tag filtering: %w", err)
		}
		return meta, nil
	}

	for _, r := range resp.Reservations {
		for _, inst := range r.Instances {
			tags := make(map[string]string, len(inst.Tags))
			for _, t := range inst.Tags {
				tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
			}
			meta[aws.ToString(inst.InstanceId)] = instanceMeta{name: nameTag(inst.Tags), tags: tags}
		}
	}

	return meta, nil
}

func nameTag(tags []ec2types.Tag) string {
	for _, t := range tags {
		if aws.ToString(t.Key) == "Name" {
			return aws.ToString(t.Value)
		}
	}
	return ""
}
