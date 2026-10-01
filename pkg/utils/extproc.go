package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// Audio conversion (ffmpeg) and PDF thumbnails (pdftoppm) run external programs over files
// that came from a request or from a URL. Before, any number of them could run at once (ten
// concurrent audio sends were ten ffmpeg processes), pdftoppm had no time limit at all, and
// a program that wrote without end filled the process's memory through the output buffer.
//
// RunLimited runs one with a time limit, a cap on its output, and a slot from a pool shared
// by every conversion of the process.

var (
	// ErrBusy: no conversion slot became free in time.
	ErrBusy = errors.New("the server is busy converting other files, try again shortly")
	// ErrOutputTooLarge: the program wrote more than it was allowed to.
	ErrOutputTooLarge = errors.New("the converted file is too large")
)

// procWait is how long a request waits for a conversion slot before giving up.
const procWait = 30 * time.Second

// stderrKeep is how much of the program's error output is kept: its tail, which is where
// ffmpeg prints the duration of what it converted.
const stderrKeep = 64 << 10

var (
	slotsMu sync.Mutex
	slots   = make(chan struct{}, defaultSlots())
)

func defaultSlots() int {
	if v, err := strconv.Atoi(os.Getenv("MAX_CONCURRENT_CONVERSIONS")); err == nil && v > 0 {
		return v
	}
	return max(2, runtime.NumCPU()/2)
}

// SetMaxConcurrentConversions resizes the pool (MAX_CONCURRENT_CONVERSIONS sets the initial
// size). Conversions already running keep their slot.
func SetMaxConcurrentConversions(n int) {
	if n < 1 {
		n = 1
	}
	slotsMu.Lock()
	slots = make(chan struct{}, n)
	slotsMu.Unlock()
}

func currentSlots() chan struct{} {
	slotsMu.Lock()
	defer slotsMu.Unlock()
	return slots
}

// limitedBuffer collects at most max bytes and then fails the write, which makes the
// program stop (its stdout pipe is broken).
type limitedBuffer struct {
	buf      bytes.Buffer
	max      int64
	exceeded bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if int64(l.buf.Len()+len(p)) > l.max {
		l.exceeded = true
		return 0, ErrOutputTooLarge
	}
	return l.buf.Write(p)
}

// tailBuffer keeps the last n bytes written to it.
type tailBuffer struct {
	b []byte
	n int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > 2*t.n {
		t.b = append(t.b[:0], t.b[len(t.b)-t.n:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) Bytes() []byte {
	if len(t.b) > t.n {
		return t.b[len(t.b)-t.n:]
	}
	return t.b
}

// RunLimited runs name with args, feeding it stdin, and returns what it wrote to stdout and
// the tail of stderr. It gives up after timeout (the process is killed), fails with
// ErrOutputTooLarge when stdout exceeds maxStdout bytes, and waits up to 30 s for a slot of
// the shared pool (ErrBusy otherwise). stderr is returned even on failure, for the message.
func RunLimited(ctx context.Context, timeout time.Duration, name string, args []string, stdin []byte, maxStdout int64) (stdout, stderr []byte, err error) {
	pool := currentSlots()
	wait := time.NewTimer(procWait)
	defer wait.Stop()
	select {
	case pool <- struct{}{}:
		defer func() { <-pool }()
	case <-wait.C:
		return nil, nil, ErrBusy
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.WaitDelay = 5 * time.Second // do not wait on pipes held open by a killed process
	cmd.Stdin = bytes.NewReader(stdin)
	out := &limitedBuffer{max: maxStdout}
	errTail := &tailBuffer{n: stderrKeep}
	cmd.Stdout = out
	cmd.Stderr = errTail

	runErr := cmd.Run()
	switch {
	case runCtx.Err() == context.DeadlineExceeded:
		return nil, errTail.Bytes(), fmt.Errorf("%s timed out after %v", name, timeout)
	case out.exceeded:
		return nil, errTail.Bytes(), ErrOutputTooLarge
	case runErr != nil:
		return nil, errTail.Bytes(), runErr
	}
	return out.buf.Bytes(), errTail.Bytes(), nil
}
