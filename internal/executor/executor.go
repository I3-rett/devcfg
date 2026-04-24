package executor

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
)

type Result struct {
	Output string
	Err    error
}

// Execute runs a command and captures combined stdout+stderr.
func Execute(args []string) Result {
	return ExecuteWithContext(context.Background(), args, nil)
}

// ExecuteWithContext runs a command with the given context, capturing combined
// stdout+stderr. If logCh is non-nil, each line of output is forwarded to it
// as the command runs; lines are dropped (not buffered) when the channel is
// full so the command is never blocked by a slow consumer. The caller is
// responsible for closing logCh after ExecuteWithContext returns.
func ExecuteWithContext(ctx context.Context, args []string, logCh chan<- string) Result {
	if len(args) == 0 {
		return Result{}
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	defer pr.Close() // ensure the read end is released when the function exits

	if err := cmd.Start(); err != nil {
		_ = pw.Close()
		_ = pr.Close()
		return Result{Err: err}
	}

	var buf bytes.Buffer
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			buf.WriteString(line + "\n")
			if logCh != nil {
				// Non-blocking send: drop the line rather than deadlock when
				// the channel buffer is full and the consumer is slow.
				select {
				case logCh <- line:
				default:
				}
			}
		}
	}()

	err := cmd.Wait()
	_ = pw.Close() // signals EOF to the scanner goroutine
	<-scanDone     // wait for all lines to be forwarded before returning

	return Result{Output: buf.String(), Err: err}
}
