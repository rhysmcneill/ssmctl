package ssm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// perInstanceRunClient answers SendCommand/GetCommandInvocation keyed by
// instance ID so it can be shared safely between concurrent runs.
type perInstanceRunClient struct {
	mu        sync.Mutex
	documents map[string]string
	sendErr   map[string]error
	stdout    map[string]string
	exitCode  map[string]int32
	delay     time.Duration
	delays    map[string]time.Duration

	inFlight    atomic.Int32
	maxInFlight atomic.Int32
}

func (c *perInstanceRunClient) SendCommand(_ context.Context, in *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	id := in.InstanceIds[0]

	c.mu.Lock()
	if c.documents == nil {
		c.documents = map[string]string{}
	}
	c.documents[id] = aws.ToString(in.DocumentName)
	c.mu.Unlock()

	if err := c.sendErr[id]; err != nil {
		return nil, err
	}

	n := c.inFlight.Add(1)
	for {
		prev := c.maxInFlight.Load()
		if n <= prev || c.maxInFlight.CompareAndSwap(prev, n) {
			break
		}
	}
	time.Sleep(c.delay + c.delays[id])

	return &ssm.SendCommandOutput{Command: &types.Command{CommandId: aws.String("cmd-" + id)}}, nil
}

func (c *perInstanceRunClient) GetCommandInvocation(_ context.Context, in *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	id := aws.ToString(in.InstanceId)
	c.inFlight.Add(-1)
	return &ssm.GetCommandInvocationOutput{
		Status:                types.CommandInvocationStatusSuccess,
		StandardOutputContent: aws.String(c.stdout[id]),
		ResponseCode:          c.exitCode[id],
	}, nil
}

func echoCommand(TargetInfo) []string { return []string{"echo hi"} }

func TestRunCommandOnInstances_PreservesOrderAndResults(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	client := &perInstanceRunClient{
		stdout:   map[string]string{"i-1": "one\n", "i-2": "two\n", "i-3": "three\n"},
		exitCode: map[string]int32{"i-2": 3},
	}
	instances := []InstanceInfo{{InstanceID: "i-1"}, {InstanceID: "i-2"}, {InstanceID: "i-3"}}

	results := RunCommandOnInstances(context.Background(), client, instances, echoCommand, 30*time.Second, 0, nil)

	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	for i, want := range []struct {
		id, stdout string
		code       int
	}{{"i-1", "one\n", 0}, {"i-2", "two\n", 3}, {"i-3", "three\n", 0}} {
		r := results[i]
		if r.Err != nil {
			t.Fatalf("results[%d].Err = %v", i, r.Err)
		}
		if r.Instance.InstanceID != want.id || r.Result.Stdout != want.stdout || r.Result.ExitCode != want.code {
			t.Errorf("results[%d] = {%s %q %d}, want {%s %q %d}", i,
				r.Instance.InstanceID, r.Result.Stdout, r.Result.ExitCode, want.id, want.stdout, want.code)
		}
	}
}

func TestRunCommandOnInstances_ChoosesDocumentPerPlatform(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	client := &perInstanceRunClient{}
	instances := []InstanceInfo{
		{InstanceID: "i-linux", Platform: "Linux"},
		{InstanceID: "i-win", Platform: "Windows"},
	}

	var gotWindows sync.Map
	commandFor := func(t TargetInfo) []string {
		gotWindows.Store(t.InstanceID, t.IsWindows())
		return []string{"cmd"}
	}

	RunCommandOnInstances(context.Background(), client, instances, commandFor, 30*time.Second, 0, nil)

	if got := client.documents["i-linux"]; got != runShellDocument {
		t.Errorf("linux document = %q, want %q", got, runShellDocument)
	}
	if got := client.documents["i-win"]; got != runPowerShellDocument {
		t.Errorf("windows document = %q, want %q", got, runPowerShellDocument)
	}
	if v, _ := gotWindows.Load("i-win"); v != true {
		t.Errorf("commandFor received IsWindows() = %v for Windows instance, want true", v)
	}
}

func TestRunCommandOnInstances_ErrorOnOneInstanceDoesNotStopOthers(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	client := &perInstanceRunClient{
		sendErr: map[string]error{"i-bad": errors.New("InvalidInstanceId")},
		stdout:  map[string]string{"i-good": "ok\n"},
	}
	instances := []InstanceInfo{{InstanceID: "i-bad"}, {InstanceID: "i-good"}}

	results := RunCommandOnInstances(context.Background(), client, instances, echoCommand, 30*time.Second, 0, nil)

	if results[0].Err == nil {
		t.Error("results[0].Err = nil, want send error")
	}
	if results[1].Err != nil || results[1].Result.Stdout != "ok\n" {
		t.Errorf("results[1] = %+v, want successful result", results[1])
	}
}

func TestRunCommandOnInstances_RespectsConcurrencyLimit(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	client := &perInstanceRunClient{delay: 20 * time.Millisecond}
	instances := make([]InstanceInfo, 6)
	for i := range instances {
		instances[i] = InstanceInfo{InstanceID: "i-" + string(rune('a'+i))}
	}

	results := RunCommandOnInstances(context.Background(), client, instances, echoCommand, 30*time.Second, 2, nil)

	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("results[%d].Err = %v", i, r.Err)
		}
	}
	if got := client.maxInFlight.Load(); got > 2 {
		t.Errorf("max in-flight = %d, want <= 2", got)
	}
}

func TestRunCommandOnInstances_OnDoneReportsInCompletionOrder(t *testing.T) {
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = 2 * time.Second })

	client := &perInstanceRunClient{
		delays: map[string]time.Duration{"i-slow": 200 * time.Millisecond},
	}
	instances := []InstanceInfo{{InstanceID: "i-slow"}, {InstanceID: "i-fast"}}

	var order []string
	results := RunCommandOnInstances(context.Background(), client, instances, echoCommand, 30*time.Second, 0, func(r TargetResult) {
		order = append(order, r.Instance.InstanceID)
	})

	if len(order) != 2 || order[0] != "i-fast" || order[1] != "i-slow" {
		t.Errorf("onDone order = %v, want [i-fast i-slow]", order)
	}
	if results[0].Instance.InstanceID != "i-slow" || results[1].Instance.InstanceID != "i-fast" {
		t.Errorf("returned results not in input order: %s, %s", results[0].Instance.InstanceID, results[1].Instance.InstanceID)
	}
}

func TestRunCommandOnInstances_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &perInstanceRunClient{}
	results := RunCommandOnInstances(ctx, client, []InstanceInfo{{InstanceID: "i-1"}}, echoCommand, 30*time.Second, 1, nil)

	if !errors.Is(results[0].Err, context.Canceled) {
		t.Errorf("Err = %v, want context.Canceled", results[0].Err)
	}
}

func TestInstanceInfo_TargetInfoAndLabel(t *testing.T) {
	tests := []struct {
		name        string
		info        InstanceInfo
		wantWindows bool
		wantLabel   string
	}{
		{"linux named", InstanceInfo{InstanceID: "i-1", Name: "web-1", Platform: "Linux"}, false, "web-1"},
		{"windows unnamed", InstanceInfo{InstanceID: "i-2", Platform: "Windows"}, true, "i-2"},
		{"macos", InstanceInfo{InstanceID: "i-3", Platform: "MacOS"}, false, "i-3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ti := tc.info.TargetInfo()
			if ti.InstanceID != tc.info.InstanceID {
				t.Errorf("InstanceID = %q, want %q", ti.InstanceID, tc.info.InstanceID)
			}
			if ti.IsWindows() != tc.wantWindows {
				t.Errorf("IsWindows() = %v, want %v", ti.IsWindows(), tc.wantWindows)
			}
			if tc.wantWindows && ti.Platform != ec2types.PlatformValuesWindows {
				t.Errorf("Platform = %q, want %q", ti.Platform, ec2types.PlatformValuesWindows)
			}
			if got := tc.info.Label(); got != tc.wantLabel {
				t.Errorf("Label() = %q, want %q", got, tc.wantLabel)
			}
		})
	}
}
