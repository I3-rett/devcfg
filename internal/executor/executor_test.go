package executor

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestExecute_EmptyArgs(t *testing.T) {
	res := Execute([]string{})
	if res.Err != nil {
		t.Errorf("Execute(empty) Err = %v; want nil", res.Err)
	}
	if res.Output != "" {
		t.Errorf("Execute(empty) Output = %q; want empty string", res.Output)
	}
}

func TestExecute_Echo(t *testing.T) {
	res := Execute([]string{"echo", "hello devcfg"})
	if res.Err != nil {
		t.Fatalf("Execute(echo) Err = %v; want nil", res.Err)
	}
	if !strings.Contains(res.Output, "hello devcfg") {
		t.Errorf("Execute(echo) Output = %q; want it to contain %q", res.Output, "hello devcfg")
	}
}

func TestExecute_FailingCommand(t *testing.T) {
	res := Execute([]string{"false"})
	if res.Err == nil {
		t.Error("Execute(false) Err = nil; want non-nil error for failing command")
	}
}

func TestExecute_NonExistentBinary(t *testing.T) {
	res := Execute([]string{"_devcfg_nonexistent_binary_xyz"})
	if res.Err == nil {
		t.Error("Execute(nonexistent) Err = nil; want non-nil error")
	}
}

func TestExecute_CapturesStderr(t *testing.T) {
	// `sh -c 'echo err >&2'` writes to stderr; executor must capture it
	res := Execute([]string{"sh", "-c", "echo stderr_output >&2"})
	if !strings.Contains(res.Output, "stderr_output") {
		t.Errorf("Execute did not capture stderr; Output = %q", res.Output)
	}
}

func TestExecute_CombinesStdoutAndStderr(t *testing.T) {
	res := Execute([]string{"sh", "-c", "echo out && echo err >&2"})
	if !strings.Contains(res.Output, "out") {
		t.Errorf("Output missing stdout content; got %q", res.Output)
	}
	if !strings.Contains(res.Output, "err") {
		t.Errorf("Output missing stderr content; got %q", res.Output)
	}
}

func TestExecuteWithContext_LogChannelReceivesLines(t *testing.T) {
	logCh := make(chan string, 16)
	res := ExecuteWithContext(context.Background(), []string{"printf", "line1\\nline2\\nline3\\n"}, logCh)
	// ExecuteWithContext returns only after all lines have been forwarded.
	close(logCh)
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	var lines []string
	for l := range logCh {
		lines = append(lines, l)
	}
	if len(lines) != 3 {
		t.Errorf("got %d lines on logCh; want 3: %v", len(lines), lines)
	}
	for i, want := range []string{"line1", "line2", "line3"} {
		if i >= len(lines) || lines[i] != want {
			t.Errorf("lines[%d] = %q; want %q", i, lines[i], want)
		}
	}
}

func TestExecuteWithContext_CancellationTerminatesProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan Result, 1)
	go func() {
		done <- ExecuteWithContext(ctx, []string{"sleep", "60"}, nil)
	}()
	// Cancel immediately; the process should be killed and the call return quickly.
	cancel()
	select {
	case res := <-done:
		if res.Err == nil {
			t.Error("expected non-nil error after context cancellation; got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ExecuteWithContext did not return within 5s after context cancellation")
	}
}
