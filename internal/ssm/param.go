package ssm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ParamAPI is the subset of ssm.Client used by the Parameter Store helpers.
// *ssm.Client satisfies this interface automatically.
type ParamAPI interface {
	GetParameter(ctx context.Context, in *awsssm.GetParameterInput, opts ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error)
	GetParametersByPath(ctx context.Context, in *awsssm.GetParametersByPathInput, opts ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error)
	PutParameter(ctx context.Context, in *awsssm.PutParameterInput, opts ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error)
	DeleteParameter(ctx context.Context, in *awsssm.DeleteParameterInput, opts ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error)
}

// Parameter is the JSON-serialisable view of an SSM parameter.
type Parameter struct {
	Name             string `json:"name"`
	Type             string `json:"type"`
	Value            string `json:"value"`
	Version          int64  `json:"version"`
	ARN              string `json:"arn,omitempty"`
	LastModifiedDate string `json:"last_modified_date,omitempty"`
}

// PutParameterResult is the JSON-serialisable result of a successful param put.
type PutParameterResult struct {
	Name    string `json:"name"`
	Version int64  `json:"version"`
	Tier    string `json:"tier,omitempty"`
}

// GetParameter fetches a single parameter, automatically decrypting SecureString values.
func GetParameter(ctx context.Context, client ParamAPI, name string) (*Parameter, error) {
	resp, err := client.GetParameter(ctx, &awsssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("get parameter: %w", err)
	}

	p := resp.Parameter
	return &Parameter{
		Name:             aws.ToString(p.Name),
		Type:             string(p.Type),
		Value:            aws.ToString(p.Value),
		Version:          p.Version,
		ARN:              aws.ToString(p.ARN),
		LastModifiedDate: formatParamTime(p.LastModifiedDate),
	}, nil
}

// ListParameters returns all parameters under a path prefix, optionally recursing into sub-paths.
// Results are paginated automatically.
func ListParameters(ctx context.Context, client ParamAPI, path string, recursive bool) ([]Parameter, error) {
	var result []Parameter
	var nextToken *string

	for {
		resp, err := client.GetParametersByPath(ctx, &awsssm.GetParametersByPathInput{
			Path:           aws.String(path),
			Recursive:      aws.Bool(recursive),
			WithDecryption: aws.Bool(true),
			NextToken:      nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list parameters: %w", err)
		}

		for _, p := range resp.Parameters {
			result = append(result, Parameter{
				Name:             aws.ToString(p.Name),
				Type:             string(p.Type),
				Value:            aws.ToString(p.Value),
				Version:          p.Version,
				ARN:              aws.ToString(p.ARN),
				LastModifiedDate: formatParamTime(p.LastModifiedDate),
			})
		}

		if resp.NextToken == nil {
			break
		}
		nextToken = resp.NextToken
	}

	return result, nil
}

// PutParameter creates or updates an SSM parameter. If the parameter already exists and
// overwrite is false, a friendly error is returned suggesting the caller use --overwrite.
func PutParameter(ctx context.Context, client ParamAPI, name, value, paramType string, overwrite bool) (*PutParameterResult, error) {
	resp, err := client.PutParameter(ctx, &awsssm.PutParameterInput{
		Name:      aws.String(name),
		Value:     aws.String(value),
		Type:      types.ParameterType(paramType),
		Overwrite: aws.Bool(overwrite),
	})
	if err != nil {
		var alreadyExists *types.ParameterAlreadyExists
		if errors.As(err, &alreadyExists) {
			return nil, fmt.Errorf("parameter %q already exists — use --overwrite to update it", name)
		}
		return nil, fmt.Errorf("put parameter: %w", err)
	}

	return &PutParameterResult{
		Name:    name,
		Version: resp.Version,
		Tier:    string(resp.Tier),
	}, nil
}

// DeleteParameter removes an SSM parameter by name.
func DeleteParameter(ctx context.Context, client ParamAPI, name string) error {
	if _, err := client.DeleteParameter(ctx, &awsssm.DeleteParameterInput{
		Name: aws.String(name),
	}); err != nil {
		return fmt.Errorf("delete parameter: %w", err)
	}
	return nil
}

func formatParamTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}
