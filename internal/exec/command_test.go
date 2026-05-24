package exec

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
)

// TestMain lets a test re-exec this binary as a child process that exits with a
// chosen code, so the exit-code mapping can be covered without any host tool.
func TestMain(m *testing.M) {
	if code := os.Getenv("EXEC_TEST_EXIT"); code != "" {
		n, _ := strconv.Atoi(code)
		os.Exit(n)
	}

	os.Exit(m.Run())
}

func TestRunDeniedByGate(t *testing.T) {
	SetGate(denyAll)

	_, err := Run(context.Background(), "anything", "--version")

	if !errors.Is(err, ErrCommandNotAllowed) {
		t.Fatalf("got %v, want ErrCommandNotAllowed", err)
	}
}

func TestRunCommandNotFound(t *testing.T) {
	SetGate(func(string) bool { return true })
	t.Cleanup(func() { SetGate(denyAll) })

	_, err := Run(context.Background(), "preflight-nonexistent-binary-xyz")

	if err == nil || errors.Is(err, ErrCommandNotAllowed) {
		t.Fatalf("got %v, want a command-not-found error", err)
	}
}

func TestRunNonzeroExitReturnsCommandError(t *testing.T) {
	SetGate(func(string) bool { return true })
	t.Cleanup(func() { SetGate(denyAll) })
	t.Setenv("EXEC_TEST_EXIT", "3")

	_, err := Run(context.Background(), os.Args[0])

	var cmdErr *CommandError

	if !errors.As(err, &cmdErr) {
		t.Fatalf("got %v, want *CommandError", err)
	}

	if cmdErr.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", cmdErr.ExitCode)
	}
}

func TestRunStreamingNonzeroExitReturnsCommandError(t *testing.T) {
	SetGate(func(string) bool { return true })
	t.Cleanup(func() { SetGate(denyAll) })
	t.Setenv("EXEC_TEST_EXIT", "4")

	_, err := RunStreamingInDir(context.Background(), "", os.Args[0], nil, nil, nil)

	var cmdErr *CommandError

	if !errors.As(err, &cmdErr) {
		t.Fatalf("got %v, want *CommandError", err)
	}

	if cmdErr.ExitCode != 4 {
		t.Errorf("exit code = %d, want 4", cmdErr.ExitCode)
	}
}
