package executor

import (
	"strings"
	"testing"
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
