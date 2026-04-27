// Package e2e contains end-to-end tests that compile and exercise the ssmctl
// binary as a subprocess.
//
// Smoke tests (no AWS required) run with the default build tags and are
// executed by `make e2e` and in CI on every PR.
//
// AWS integration tests require real credentials and are guarded by the "e2e"
// build tag.  Run them with `make e2e-aws`.
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// binaryPath holds the path to the compiled ssmctl binary built by TestMain.
var binaryPath string

// TestMain builds the ssmctl binary once before running all tests in this
// package, and cleans it up afterwards.
func TestMain(m *testing.M) {
	bin, cleanup, err := buildBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: failed to build binary: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	binaryPath = bin
	//nolint:gocritic // os.Exit(m.Run()) is the standard TestMain pattern.
	os.Exit(m.Run())
}

// buildBinary compiles cmd/ssmctl into a temporary directory and returns the
// path to the binary together with a cleanup function.
func buildBinary() (string, func(), error) {
	dir, err := os.MkdirTemp("", "ssmctl-e2e-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp directory: %w", err)
	}

	cleanup := func() { _ = os.RemoveAll(dir) }

	bin := filepath.Join(dir, "ssmctl")

	// Resolve the module root relative to this file so the test works from any
	// working directory.
	moduleRoot, err := moduleRoot()
	if err != nil {
		cleanup()
		return "", nil, err
	}

	cmd := exec.Command("go", "build", "-o", bin, "./cmd/ssmctl")
	cmd.Dir = moduleRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("go build: %w\n%s", err, out)
	}

	return bin, cleanup, nil
}

// moduleRoot walks up from this file to find the directory containing go.mod.
func moduleRoot() (string, error) {
	// __file__ is the directory of this source file at compile time — we use
	// os.Getwd() as a fallback since the test binary runs from the package dir.
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod starting from %s", dir)
		}
		dir = parent
	}
}

// run executes the ssmctl binary with the supplied args and returns its
// combined stdout+stderr output and exit error (nil on exit code 0).
func run(args ...string) (string, error) {
	cmd := exec.Command(binaryPath, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
