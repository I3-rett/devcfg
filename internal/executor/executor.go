package executor

import (
	"bytes"
	"os/exec"
)

type Result struct {
	Output string
	Err    error
}

// Execute runs a command and captures combined stdout+stderr.
func Execute(args []string) Result {
	if len(args) == 0 {
		return Result{Err: nil, Output: ""}
	}
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return Result{
		Output: buf.String(),
		Err:    err,
	}
}
