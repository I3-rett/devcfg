package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
)

// Result holds the combined output and error of a command execution.
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
	defer func() { _ = pr.Close() }() // ensure the read end is released when the function exits

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
		// Set a 1 MiB per-line buffer so tools that emit very long lines
		// (e.g. base64-encoded artifacts) don't cause Scan() to stop early
		// and potentially leave unread data blocking cmd.Wait().
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
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
		if err := scanner.Err(); err != nil {
			// Surface read errors (e.g. a line that exceeds the buffer) as an
			// inline log entry so they're visible and not silently swallowed.
			errLine := "⚠ output read error: " + err.Error()
			buf.WriteString(errLine + "\n")
			if logCh != nil {
				select {
				case logCh <- errLine:
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

// emitPendingLines processes the pending buffer, emitting each complete line
// via emit. It returns the remaining bytes that do not yet form a full line.
func emitPendingLines(pending []byte, emit func(string)) []byte {
	for {
		idx := bytes.IndexAny(pending, "\r\n")
		if idx < 0 {
			break
		}
		emit(string(pending[:idx]))
		next := idx + 1
		if pending[idx] == '\r' && next < len(pending) && pending[next] == '\n' {
			next++
		}
		pending = pending[next:]
	}
	return pending
}

// ExecuteWithPTY starts a command attached to a new pseudo-terminal so that
// interactive programs (sudo, brew, etc.) can prompt for input naturally.
// It returns the PTY master file (read for output, write for input), a channel
// that receives the process exit error after all output has been forwarded to
// logCh, and any startup error.  The caller must close ptm after the error
// channel delivers a value.  logCh is closed by this function after all output
// has been forwarded; the caller must not close it.
// Returns an error immediately when args is empty.
func ExecuteWithPTY(ctx context.Context, args []string, logCh chan<- string) (ptm *os.File, errCh <-chan error, err error) {
	if len(args) == 0 {
		return nil, nil, fmt.Errorf("ExecuteWithPTY: no command provided")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec
	ptm, err = pty.Start(cmd)
	if err != nil {
		return nil, nil, err
	}

	emit := func(s string) {
		// Preserve ANSI color codes but strip bare carriage returns
		s = strings.ReplaceAll(s, "\r", "")
		s = strings.TrimRight(s, " \t\r\n")
		if s != "" && logCh != nil {
			select {
			case logCh <- s:
			default:
			}
		}
	}

	ch := make(chan error, 1)
	go func() {
		readBuf := make([]byte, 4096)
		var pending []byte
		for {
			n, readErr := ptm.Read(readBuf)
			if n > 0 {
				pending = append(pending, readBuf[:n]...)
				pending = emitPendingLines(pending, emit)
				// Emit partial content immediately (e.g. "Password: " prompts).
				if len(pending) > 0 {
					emit(string(pending))
					pending = pending[:0]
				}
			}
			if readErr != nil {
				if len(pending) > 0 {
					emit(string(pending))
				}
				ch <- cmd.Wait()
				if logCh != nil {
					close(logCh)
				}
				return
			}
		}
	}()
	return ptm, ch, nil
}
