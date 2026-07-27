package ssm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// mockParamAPI implements ParamAPI for tests.
type mockParamAPI struct {
	getParameter        func(context.Context, *awsssm.GetParameterInput, ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error)
	getParametersByPath func(context.Context, *awsssm.GetParametersByPathInput, ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error)
	putParameter        func(context.Context, *awsssm.PutParameterInput, ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error)
	deleteParameter     func(context.Context, *awsssm.DeleteParameterInput, ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error)
}

func (m *mockParamAPI) GetParameter(ctx context.Context, in *awsssm.GetParameterInput, opts ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error) {
	return m.getParameter(ctx, in, opts...)
}

func (m *mockParamAPI) GetParametersByPath(ctx context.Context, in *awsssm.GetParametersByPathInput, opts ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error) {
	return m.getParametersByPath(ctx, in, opts...)
}

func (m *mockParamAPI) PutParameter(ctx context.Context, in *awsssm.PutParameterInput, opts ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error) {
	return m.putParameter(ctx, in, opts...)
}

func (m *mockParamAPI) DeleteParameter(ctx context.Context, in *awsssm.DeleteParameterInput, opts ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error) {
	return m.deleteParameter(ctx, in, opts...)
}

// ── GetParameter ─────────────────────────────────────────────────────────────

func TestGetParameter_Success(t *testing.T) {
	lastMod := time.Date(2026, 7, 20, 9, 12, 0, 0, time.UTC)
	client := &mockParamAPI{
		getParameter: func(_ context.Context, in *awsssm.GetParameterInput, _ ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error) {
			if aws.ToString(in.Name) != "/myapp/prod/DB_PASSWORD" {
				t.Errorf("unexpected parameter name: %s", aws.ToString(in.Name))
			}
			if !aws.ToBool(in.WithDecryption) {
				t.Error("expected WithDecryption to be true")
			}
			return &awsssm.GetParameterOutput{
				Parameter: &types.Parameter{
					Name:             aws.String("/myapp/prod/DB_PASSWORD"),
					Type:             types.ParameterTypeSecureString,
					Value:            aws.String("supersecretpassword"),
					Version:          3,
					ARN:              aws.String("arn:aws:ssm:eu-west-1:123:parameter/myapp/prod/DB_PASSWORD"),
					LastModifiedDate: &lastMod,
				},
			}, nil
		},
	}

	param, err := GetParameter(context.Background(), client, "/myapp/prod/DB_PASSWORD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if param.Value != "supersecretpassword" {
		t.Errorf("Value = %q, want %q", param.Value, "supersecretpassword")
	}
	if param.Type != "SecureString" {
		t.Errorf("Type = %q, want %q", param.Type, "SecureString")
	}
	if param.Version != 3 {
		t.Errorf("Version = %d, want 3", param.Version)
	}
	if param.LastModifiedDate != "2026-07-20 09:12:00" {
		t.Errorf("LastModifiedDate = %q, want %q", param.LastModifiedDate, "2026-07-20 09:12:00")
	}
}

func TestGetParameter_APIError(t *testing.T) {
	client := &mockParamAPI{
		getParameter: func(_ context.Context, _ *awsssm.GetParameterInput, _ ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error) {
			return nil, errors.New("connection refused")
		},
	}

	_, err := GetParameter(context.Background(), client, "/myapp/prod/MISSING")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── ListParameters ────────────────────────────────────────────────────────────

func TestListParameters_SinglePage(t *testing.T) {
	client := &mockParamAPI{
		getParametersByPath: func(_ context.Context, in *awsssm.GetParametersByPathInput, _ ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error) {
			if aws.ToString(in.Path) != "/myapp/prod/" {
				t.Errorf("unexpected path: %s", aws.ToString(in.Path))
			}
			if aws.ToBool(in.Recursive) {
				t.Error("expected Recursive to be false")
			}
			if !aws.ToBool(in.WithDecryption) {
				t.Error("expected WithDecryption to be true")
			}
			return &awsssm.GetParametersByPathOutput{
				Parameters: []types.Parameter{
					{Name: aws.String("/myapp/prod/DB_PASSWORD"), Type: types.ParameterTypeSecureString, Value: aws.String("secret"), Version: 3},
					{Name: aws.String("/myapp/prod/API_URL"), Type: types.ParameterTypeString, Value: aws.String("https://api.example.com"), Version: 2},
				},
			}, nil
		},
	}

	params, err := ListParameters(context.Background(), client, "/myapp/prod/", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(params))
	}
	if params[0].Name != "/myapp/prod/DB_PASSWORD" {
		t.Errorf("params[0].Name = %q, want %q", params[0].Name, "/myapp/prod/DB_PASSWORD")
	}
}

func TestListParameters_Paginated(t *testing.T) {
	callCount := 0
	client := &mockParamAPI{
		getParametersByPath: func(_ context.Context, in *awsssm.GetParametersByPathInput, _ ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error) {
			callCount++
			switch callCount {
			case 1:
				return &awsssm.GetParametersByPathOutput{
					Parameters: []types.Parameter{
						{Name: aws.String("/myapp/prod/A"), Type: types.ParameterTypeString, Value: aws.String("a"), Version: 1},
					},
					NextToken: aws.String("token-page-2"),
				}, nil
			case 2:
				if aws.ToString(in.NextToken) != "token-page-2" {
					t.Errorf("expected NextToken %q, got %q", "token-page-2", aws.ToString(in.NextToken))
				}
				return &awsssm.GetParametersByPathOutput{
					Parameters: []types.Parameter{
						{Name: aws.String("/myapp/prod/B"), Type: types.ParameterTypeString, Value: aws.String("b"), Version: 1},
					},
				}, nil
			default:
				t.Errorf("unexpected call count: %d", callCount)
				return &awsssm.GetParametersByPathOutput{}, nil
			}
		},
	}

	params, err := ListParameters(context.Background(), client, "/myapp/prod/", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params) != 2 {
		t.Fatalf("expected 2 parameters across 2 pages, got %d", len(params))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestListParameters_Recursive(t *testing.T) {
	client := &mockParamAPI{
		getParametersByPath: func(_ context.Context, in *awsssm.GetParametersByPathInput, _ ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error) {
			if !aws.ToBool(in.Recursive) {
				t.Error("expected Recursive to be true")
			}
			return &awsssm.GetParametersByPathOutput{}, nil
		},
	}

	if _, err := ListParameters(context.Background(), client, "/myapp/", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListParameters_Empty(t *testing.T) {
	client := &mockParamAPI{
		getParametersByPath: func(_ context.Context, _ *awsssm.GetParametersByPathInput, _ ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error) {
			return &awsssm.GetParametersByPathOutput{}, nil
		},
	}

	params, err := ListParameters(context.Background(), client, "/empty/path/", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(params) != 0 {
		t.Errorf("expected 0 parameters, got %d", len(params))
	}
}

func TestListParameters_APIError(t *testing.T) {
	client := &mockParamAPI{
		getParametersByPath: func(_ context.Context, _ *awsssm.GetParametersByPathInput, _ ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	_, err := ListParameters(context.Background(), client, "/myapp/prod/", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── PutParameter ──────────────────────────────────────────────────────────────

func TestPutParameter_Success(t *testing.T) {
	client := &mockParamAPI{
		putParameter: func(_ context.Context, in *awsssm.PutParameterInput, _ ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error) {
			if aws.ToString(in.Name) != "/myapp/prod/DB_PASSWORD" {
				t.Errorf("unexpected name: %s", aws.ToString(in.Name))
			}
			if aws.ToString(in.Value) != "newvalue" {
				t.Errorf("unexpected value: %s", aws.ToString(in.Value))
			}
			if in.Type != types.ParameterTypeSecureString {
				t.Errorf("unexpected type: %s", in.Type)
			}
			if !aws.ToBool(in.Overwrite) {
				t.Error("expected Overwrite to be true")
			}
			return &awsssm.PutParameterOutput{Version: 4, Tier: types.ParameterTierStandard}, nil
		},
	}

	result, err := PutParameter(context.Background(), client, "/myapp/prod/DB_PASSWORD", "newvalue", "SecureString", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Version != 4 {
		t.Errorf("Version = %d, want 4", result.Version)
	}
	if result.Name != "/myapp/prod/DB_PASSWORD" {
		t.Errorf("Name = %q, want %q", result.Name, "/myapp/prod/DB_PASSWORD")
	}
}

func TestPutParameter_AlreadyExists_FriendlyError(t *testing.T) {
	client := &mockParamAPI{
		putParameter: func(_ context.Context, _ *awsssm.PutParameterInput, _ ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error) {
			return nil, &types.ParameterAlreadyExists{Message: aws.String("parameter already exists")}
		},
	}

	_, err := PutParameter(context.Background(), client, "/myapp/prod/DB_PASSWORD", "value", "String", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsString(err.Error(), "--overwrite") {
		t.Errorf("expected error to mention --overwrite, got: %s", err.Error())
	}
}

func TestPutParameter_OtherAPIError(t *testing.T) {
	client := &mockParamAPI{
		putParameter: func(_ context.Context, _ *awsssm.PutParameterInput, _ ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	_, err := PutParameter(context.Background(), client, "/myapp/prod/DB_PASSWORD", "value", "String", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── DeleteParameter ───────────────────────────────────────────────────────────

func TestDeleteParameter_Success(t *testing.T) {
	client := &mockParamAPI{
		deleteParameter: func(_ context.Context, in *awsssm.DeleteParameterInput, _ ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error) {
			if aws.ToString(in.Name) != "/myapp/prod/DB_PASSWORD" {
				t.Errorf("unexpected name: %s", aws.ToString(in.Name))
			}
			return &awsssm.DeleteParameterOutput{}, nil
		},
	}

	if err := DeleteParameter(context.Background(), client, "/myapp/prod/DB_PASSWORD"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteParameter_APIError(t *testing.T) {
	client := &mockParamAPI{
		deleteParameter: func(_ context.Context, _ *awsssm.DeleteParameterInput, _ ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error) {
			return nil, errors.New("parameter not found")
		},
	}

	if err := DeleteParameter(context.Background(), client, "/myapp/prod/MISSING"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
