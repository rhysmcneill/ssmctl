package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/spf13/cobra"

	"github.com/rhysmcneill/ssmctl/internal/app"
	"github.com/rhysmcneill/ssmctl/internal/config"
	"github.com/rhysmcneill/ssmctl/internal/output"
	ssmlib "github.com/rhysmcneill/ssmctl/internal/ssm"
)

// ── mock client ───────────────────────────────────────────────────────────────

type mockParamCmdClient struct {
	getParameter        func(context.Context, *awsssm.GetParameterInput, ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error)
	getParametersByPath func(context.Context, *awsssm.GetParametersByPathInput, ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error)
	putParameter        func(context.Context, *awsssm.PutParameterInput, ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error)
	deleteParameter     func(context.Context, *awsssm.DeleteParameterInput, ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error)
}

func (m *mockParamCmdClient) GetParameter(ctx context.Context, in *awsssm.GetParameterInput, opts ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error) {
	return m.getParameter(ctx, in, opts...)
}

func (m *mockParamCmdClient) GetParametersByPath(ctx context.Context, in *awsssm.GetParametersByPathInput, opts ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error) {
	return m.getParametersByPath(ctx, in, opts...)
}

func (m *mockParamCmdClient) PutParameter(ctx context.Context, in *awsssm.PutParameterInput, opts ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error) {
	return m.putParameter(ctx, in, opts...)
}

func (m *mockParamCmdClient) DeleteParameter(ctx context.Context, in *awsssm.DeleteParameterInput, opts ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error) {
	return m.deleteParameter(ctx, in, opts...)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func executeParamCmd(ctx context.Context, a *app.App, args []string, buf *bytes.Buffer) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(paramCmd())
	root.SetArgs(args)
	root.SetOut(buf)
	if a.Printer != nil {
		a.Printer.Out = buf
	}
	return root.ExecuteContext(context.WithValue(ctx, app.ContextKey{}, a)) //nolint:wrapcheck
}

func newParamApp(outputFmt string, client ssmlib.ParamAPI) *app.App {
	return &app.App{
		Config:      &config.Config{Output: outputFmt, Timeout: 30 * time.Second},
		ParamClient: client,
		Printer:     &output.Printer{Format: outputFmt},
	}
}

// ── param get ─────────────────────────────────────────────────────────────────

func TestParamGetCmd_TextOutput(t *testing.T) {
	client := &mockParamCmdClient{
		getParameter: func(_ context.Context, _ *awsssm.GetParameterInput, _ ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error) {
			return &awsssm.GetParameterOutput{
				Parameter: &types.Parameter{
					Name:    aws.String("/myapp/prod/DB_PASSWORD"),
					Type:    types.ParameterTypeSecureString,
					Value:   aws.String("supersecretpassword"),
					Version: 3,
				},
			}, nil
		},
	}

	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("text", client), []string{"param", "get", "/myapp/prod/DB_PASSWORD"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := strings.TrimSpace(buf.String())
	if out != "supersecretpassword" {
		t.Errorf("text output = %q, want %q", out, "supersecretpassword")
	}
}

func TestParamGetCmd_JSONOutput(t *testing.T) {
	lastMod := time.Date(2026, 7, 20, 9, 12, 0, 0, time.UTC)
	client := &mockParamCmdClient{
		getParameter: func(_ context.Context, _ *awsssm.GetParameterInput, _ ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error) {
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

	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("json", client), []string{"param", "get", "/myapp/prod/DB_PASSWORD"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got ssmlib.Parameter
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.Value != "supersecretpassword" {
		t.Errorf("value = %q, want %q", got.Value, "supersecretpassword")
	}
	if got.Type != "SecureString" {
		t.Errorf("type = %q, want %q", got.Type, "SecureString")
	}
	if got.Version != 3 {
		t.Errorf("version = %d, want 3", got.Version)
	}
}

func TestParamGetCmd_APIError(t *testing.T) {
	client := &mockParamCmdClient{
		getParameter: func(_ context.Context, _ *awsssm.GetParameterInput, _ ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error) {
			return nil, errors.New("ParameterNotFound")
		},
	}

	var buf bytes.Buffer
	err := executeParamCmd(context.Background(), newParamApp("text", client), []string{"param", "get", "/missing"}, &buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── param list ────────────────────────────────────────────────────────────────

// errWriter is an io.Writer that always returns an error — used to exercise
// write-failure branches in cobra RunE functions.
type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) { return 0, errors.New("write error") }

func makeListClient(params []types.Parameter) *mockParamCmdClient {
	return &mockParamCmdClient{
		getParametersByPath: func(_ context.Context, _ *awsssm.GetParametersByPathInput, _ ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error) {
			return &awsssm.GetParametersByPathOutput{Parameters: params}, nil
		},
	}
}

// executeParamCmdWithWriter is like executeParamCmd but uses a custom io.Writer
// for the cobra output, allowing write-failure tests.
func executeParamCmdWithWriter(ctx context.Context, a *app.App, args []string, w io.Writer) error {
	root := &cobra.Command{Use: "ssmctl", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(paramCmd())
	root.SetArgs(args)
	root.SetOut(w)
	if a.Printer != nil {
		a.Printer.Out = w
	}
	return root.ExecuteContext(context.WithValue(ctx, app.ContextKey{}, a)) //nolint:wrapcheck
}
func TestParamListCmd_TextOutput(t *testing.T) {
	params := []types.Parameter{
		{Name: aws.String("/myapp/prod/DB_PASSWORD"), Type: types.ParameterTypeSecureString, Value: aws.String("secret"), Version: 3},
		{Name: aws.String("/myapp/prod/API_URL"), Type: types.ParameterTypeString, Value: aws.String("https://api.example.com"), Version: 2},
	}

	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("text", makeListClient(params)), []string{"param", "list", "/myapp/prod/"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "NAME") {
		t.Errorf("output missing NAME header:\n%s", out)
	}
	if !strings.Contains(out, "/myapp/prod/DB_PASSWORD") {
		t.Errorf("output missing parameter name:\n%s", out)
	}
	if !strings.Contains(out, "SecureString") {
		t.Errorf("output missing type:\n%s", out)
	}
}

func TestParamListCmd_TextOutput_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("text", makeListClient(nil)), []string{"param", "list", "/empty/"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "NAME") {
		t.Errorf("expected header even for empty list, got:\n%s", out)
	}
}

func TestParamListCmd_JSONOutput(t *testing.T) {
	params := []types.Parameter{
		{Name: aws.String("/myapp/prod/DB_PASSWORD"), Type: types.ParameterTypeSecureString, Value: aws.String("secret"), Version: 3},
	}

	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("json", makeListClient(params)), []string{"param", "list", "/myapp/prod/"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []ssmlib.Parameter
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 parameter, got %d", len(got))
	}
	if got[0].Name != "/myapp/prod/DB_PASSWORD" {
		t.Errorf("name = %q, want %q", got[0].Name, "/myapp/prod/DB_PASSWORD")
	}
}

func TestParamListCmd_RecursiveFlag(t *testing.T) {
	var capturedRecursive bool
	client := &mockParamCmdClient{
		getParametersByPath: func(_ context.Context, in *awsssm.GetParametersByPathInput, _ ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error) {
			capturedRecursive = aws.ToBool(in.Recursive)
			return &awsssm.GetParametersByPathOutput{}, nil
		},
	}

	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("text", client), []string{"param", "list", "/myapp/", "--recursive"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capturedRecursive {
		t.Error("expected Recursive to be true when --recursive flag is set")
	}
}

func TestParamListCmd_APIError(t *testing.T) {
	client := &mockParamCmdClient{
		getParametersByPath: func(_ context.Context, _ *awsssm.GetParametersByPathInput, _ ...func(*awsssm.Options)) (*awsssm.GetParametersByPathOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	var buf bytes.Buffer
	err := executeParamCmd(context.Background(), newParamApp("text", client), []string{"param", "list", "/myapp/"}, &buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── param put ─────────────────────────────────────────────────────────────────

func TestParamPutCmd_TextOutput(t *testing.T) {
	client := &mockParamCmdClient{
		putParameter: func(_ context.Context, _ *awsssm.PutParameterInput, _ ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error) {
			return &awsssm.PutParameterOutput{Version: 4, Tier: types.ParameterTierStandard}, nil
		},
	}

	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("text", client), []string{"param", "put", "/myapp/prod/DB_PASSWORD", "newvalue", "--overwrite"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "/myapp/prod/DB_PASSWORD") {
		t.Errorf("output missing parameter name:\n%s", out)
	}
	if !strings.Contains(out, "4") {
		t.Errorf("output missing version:\n%s", out)
	}
}

func TestParamPutCmd_JSONOutput(t *testing.T) {
	client := &mockParamCmdClient{
		putParameter: func(_ context.Context, _ *awsssm.PutParameterInput, _ ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error) {
			return &awsssm.PutParameterOutput{Version: 1, Tier: types.ParameterTierStandard}, nil
		},
	}

	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("json", client), []string{"param", "put", "/myapp/prod/NEW_PARAM", "value"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got ssmlib.PutParameterResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got.Version != 1 {
		t.Errorf("version = %d, want 1", got.Version)
	}
}

func TestParamPutCmd_DefaultsToStringType(t *testing.T) {
	var capturedType types.ParameterType
	client := &mockParamCmdClient{
		putParameter: func(_ context.Context, in *awsssm.PutParameterInput, _ ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error) {
			capturedType = in.Type
			return &awsssm.PutParameterOutput{Version: 1}, nil
		},
	}

	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("text", client), []string{"param", "put", "/myapp/prod/PARAM", "value"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedType != types.ParameterTypeString {
		t.Errorf("default type = %q, want %q", capturedType, types.ParameterTypeString)
	}
}

func TestParamPutCmd_InvalidType(t *testing.T) {
	client := &mockParamCmdClient{}

	var buf bytes.Buffer
	err := executeParamCmd(context.Background(), newParamApp("text", client), []string{"param", "put", "/myapp/prod/PARAM", "value", "--type", "BadType"}, &buf)
	if err == nil {
		t.Fatal("expected error for invalid type, got nil")
	}
}

func TestParamPutCmd_AlreadyExists_FriendlyError(t *testing.T) {
	client := &mockParamCmdClient{
		putParameter: func(_ context.Context, _ *awsssm.PutParameterInput, _ ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error) {
			return nil, &types.ParameterAlreadyExists{Message: aws.String("parameter already exists")}
		},
	}

	var buf bytes.Buffer
	err := executeParamCmd(context.Background(), newParamApp("text", client), []string{"param", "put", "/myapp/prod/DB_PASSWORD", "value"}, &buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--overwrite") {
		t.Errorf("expected --overwrite hint in error, got: %s", err.Error())
	}
}

// ── param delete ──────────────────────────────────────────────────────────────

func TestParamDeleteCmd_TextOutput(t *testing.T) {
	client := &mockParamCmdClient{
		deleteParameter: func(_ context.Context, _ *awsssm.DeleteParameterInput, _ ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error) {
			return &awsssm.DeleteParameterOutput{}, nil
		},
	}

	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("text", client), []string{"param", "delete", "/myapp/prod/DB_PASSWORD"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "/myapp/prod/DB_PASSWORD") {
		t.Errorf("output missing parameter name:\n%s", out)
	}
}

func TestParamDeleteCmd_JSONOutput(t *testing.T) {
	client := &mockParamCmdClient{
		deleteParameter: func(_ context.Context, _ *awsssm.DeleteParameterInput, _ ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error) {
			return &awsssm.DeleteParameterOutput{}, nil
		},
	}

	var buf bytes.Buffer
	if err := executeParamCmd(context.Background(), newParamApp("json", client), []string{"param", "delete", "/myapp/prod/DB_PASSWORD"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
	if got["status"] != "deleted" {
		t.Errorf("status = %q, want %q", got["status"], "deleted")
	}
	if got["name"] != "/myapp/prod/DB_PASSWORD" {
		t.Errorf("name = %q, want %q", got["name"], "/myapp/prod/DB_PASSWORD")
	}
}

func TestParamDeleteCmd_APIError(t *testing.T) {
	client := &mockParamCmdClient{
		deleteParameter: func(_ context.Context, _ *awsssm.DeleteParameterInput, _ ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error) {
			return nil, errors.New("ParameterNotFound")
		},
	}

	var buf bytes.Buffer
	err := executeParamCmd(context.Background(), newParamApp("text", client), []string{"param", "delete", "/missing"}, &buf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── write-error paths ─────────────────────────────────────────────────────────

func TestParamGetCmd_WriteError(t *testing.T) {
	client := &mockParamCmdClient{
		getParameter: func(_ context.Context, _ *awsssm.GetParameterInput, _ ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error) {
			return &awsssm.GetParameterOutput{
				Parameter: &types.Parameter{
					Name:  aws.String("/myapp/prod/DB_PASSWORD"),
					Value: aws.String("secret"),
				},
			}, nil
		},
	}

	err := executeParamCmdWithWriter(context.Background(), newParamApp("text", client), []string{"param", "get", "/myapp/prod/DB_PASSWORD"}, errWriter{})
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
}

func TestParamPutCmd_WriteError(t *testing.T) {
	client := &mockParamCmdClient{
		putParameter: func(_ context.Context, _ *awsssm.PutParameterInput, _ ...func(*awsssm.Options)) (*awsssm.PutParameterOutput, error) {
			return &awsssm.PutParameterOutput{Version: 1, Tier: types.ParameterTierStandard}, nil
		},
	}

	err := executeParamCmdWithWriter(context.Background(), newParamApp("text", client), []string{"param", "put", "/myapp/prod/PARAM", "value"}, errWriter{})
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
}

func TestParamDeleteCmd_WriteError(t *testing.T) {
	client := &mockParamCmdClient{
		deleteParameter: func(_ context.Context, _ *awsssm.DeleteParameterInput, _ ...func(*awsssm.Options)) (*awsssm.DeleteParameterOutput, error) {
			return &awsssm.DeleteParameterOutput{}, nil
		},
	}

	err := executeParamCmdWithWriter(context.Background(), newParamApp("text", client), []string{"param", "delete", "/myapp/prod/PARAM"}, errWriter{})
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
}

// ── printParamTable ───────────────────────────────────────────────────────────

func TestPrintParamTable_FlushError(t *testing.T) {
	if err := printParamTable(errWriter{}, nil); err == nil {
		t.Fatal("expected flush error when underlying writer fails, got nil")
	}
}

func TestPrintParamTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := printParamTable(&buf, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "NAME") {
		t.Errorf("expected header line, got: %s", buf.String())
	}
}

func TestPrintParamTable_WithParams(t *testing.T) {
	params := []ssmlib.Parameter{
		{Name: "/myapp/prod/DB_PASSWORD", Type: "SecureString", Version: 3, LastModifiedDate: "2026-07-20 09:12:00"},
		{Name: "/myapp/prod/API_URL", Type: "String", Version: 2, LastModifiedDate: "2026-07-15 11:00:00"},
	}

	var buf bytes.Buffer
	if err := printParamTable(&buf, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"/myapp/prod/DB_PASSWORD", "SecureString", "2026-07-20 09:12:00", "/myapp/prod/API_URL"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
