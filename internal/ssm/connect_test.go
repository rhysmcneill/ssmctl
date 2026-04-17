package ssm

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func TestStartSession_EmptyRegion(t *testing.T) {
	err := StartSession(context.Background(), &ssm.Client{}, "i-1234567890abcdef0", "", "")
	if err == nil {
		t.Fatal("expected error for empty region, got nil")
	}
}
