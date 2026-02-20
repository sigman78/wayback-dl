package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"
)

// subprocessEnv is set in the re-executed subprocess so it knows to call main()
// directly instead of spawning another child.
const subprocessEnv = "WAYBACK_DL_TEST_SUBPROCESS"

// runSubprocess re-executes the test binary running only the named test,
// with subprocessEnv set so the test calls main() and lets os.Exit fire.
// Returns the *exec.ExitError (nil means exit 0).
func runSubprocess(t *testing.T, testName string) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run="+testName)
	cmd.Env = append(os.Environ(), subprocessEnv+"=1")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// TestHelpExitsZero verifies that -help prints usage and exits with code 0.
func TestHelpExitsZero(t *testing.T) {
	if os.Getenv(subprocessEnv) == "1" {
		os.Args = []string{"wayback-dl", "-help"}
		main()
		return // unreachable; main calls os.Exit
	}
	if err := runSubprocess(t, "TestHelpExitsZero"); err != nil {
		t.Fatalf("expected exit 0 for -help, got: %v", err)
	}
}

// TestUnknownFlagExitsTwo verifies that an unrecognised flag exits with code 2.
func TestUnknownFlagExitsTwo(t *testing.T) {
	if os.Getenv(subprocessEnv) == "1" {
		os.Args = []string{"wayback-dl", "-this-flag-does-not-exist"}
		main()
		return // unreachable; main calls os.Exit
	}
	err := runSubprocess(t, "TestUnknownFlagExitsTwo")
	if err == nil {
		t.Fatal("expected non-zero exit for unknown flag, got exit 0")
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
	}
}
