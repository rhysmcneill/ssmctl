package ssm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// TargetResult is the outcome of running a command on one instance as part
// of a multi-instance run. Exactly one of Result or Err is set.
type TargetResult struct {
	Instance InstanceInfo
	Result   *Result
	Err      error
}

// TargetInfo converts listed instance metadata into a TargetInfo so the
// correct Run Command document can be chosen without an extra EC2 lookup.
func (i InstanceInfo) TargetInfo() TargetInfo {
	info := TargetInfo{InstanceID: i.InstanceID}
	if strings.EqualFold(i.Platform, "windows") {
		info.Platform = ec2types.PlatformValuesWindows
	}
	return info
}

// Label returns the instance Name tag, falling back to the instance ID.
func (i InstanceInfo) Label() string {
	if i.Name != "" {
		return i.Name
	}
	return i.InstanceID
}

// RunCommandOnInstances runs a command on every instance in parallel and
// returns one TargetResult per instance, in the same order as instances.
// commandFor builds the command for each target so callers can apply
// platform-specific quoting. onDone, if non-nil, is called once per instance
// as soon as it finishes; calls are serialised so onDone need not be safe for
// concurrent use. A concurrency of zero or less means unlimited. A failure on
// one instance does not cancel the others.
func RunCommandOnInstances(ctx context.Context, client RunAPI, instances []InstanceInfo, commandFor func(TargetInfo) []string, timeout time.Duration, concurrency int, onDone func(TargetResult)) []TargetResult {
	results := make([]TargetResult, len(instances))

	if concurrency <= 0 || concurrency > len(instances) {
		concurrency = len(instances)
	}
	sem := make(chan struct{}, concurrency)

	var doneMu sync.Mutex
	finish := func(i int, r TargetResult) {
		results[i] = r
		if onDone != nil {
			doneMu.Lock()
			defer doneMu.Unlock()
			onDone(r)
		}
	}

	var wg sync.WaitGroup
	for i, inst := range instances {
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				finish(i, TargetResult{Instance: inst, Err: fmt.Errorf("context cancelled: %w", ctx.Err())})
				return
			}

			target := inst.TargetInfo()
			res, err := RunCommandForTarget(ctx, client, target, commandFor(target), timeout)
			finish(i, TargetResult{Instance: inst, Result: res, Err: err})
		}()
	}
	wg.Wait()

	return results
}
