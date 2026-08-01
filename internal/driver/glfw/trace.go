package glfw

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Run loop tracing, enabled by setting FYNE_TRACE_LOOP to any non-empty value.
//
// It exists so a bug report about an unresponsive UI can be answered with data
// instead of a guess. The condition it looks for is the run loop blocking in
// the native event wait while a mouse position update is still pending: mouse
// moves are recorded in the GLFW callback and applied later from
// processWindowEvents, so a loop that sleeps before that happens leaves hover
// state and the cursor stale until the next input event arrives. On macOS and
// Windows the OS runs a modal loop for the whole of an interactive resize, and
// the run loop cannot iterate during it, which is when the two can get out of
// step.
//
// The environment is read once at startup, so tracing costs one bool test per
// wait when it is off.
const traceEnvKey = "FYNE_TRACE_LOOP"

// traceLongWait is the sleep duration above which a wake up is reported even
// when the loop looked healthy going to sleep.
const traceLongWait = 250 * time.Millisecond

var (
	traceEnabled           = os.Getenv(traceEnvKey) != ""
	traceStart             = time.Now()
	traceOutput  io.Writer = os.Stderr
	traceLock    sync.Mutex

	// traceStaleWaits counts waits entered with a mouse update still pending.
	// This on its own is harmless and happens in normal use: an event is
	// usually already queued and the loop wakes again in microseconds.
	traceStaleWaits atomic.Uint64

	// traceStaleStalls counts the waits above that then blocked for longer
	// than traceLongWait, which is the case that is actually visible as a
	// stuck cursor and dead hover. A report that comes with a non-zero count
	// here is this bug; one that comes with a zero count is not.
	traceStaleStalls atomic.Uint64

	// traceWakes counts wakeEventLoop calls, i.e. how often something asked
	// the loop to stop sleeping. Reported alongside the waits rather than
	// logged on its own, which would drown the log during animation.
	traceWakes atomic.Uint64
)

// traceWait is the state a run loop sleep began in.
type traceWait struct {
	started time.Time
	stale   bool
}

// tracef writes one timestamped line to stderr. It is safe to call from any
// goroutine; the lock only keeps lines from interleaving.
func tracef(format string, args ...any) {
	if !traceEnabled {
		return
	}

	traceLock.Lock()
	defer traceLock.Unlock()

	fmt.Fprintf(traceOutput, "[loop %12s] %s\n",
		time.Since(traceStart).Round(time.Microsecond), fmt.Sprintf(format, args...))
}

// traceBeginWait records the state the run loop is about to block in and
// reports the stale case immediately, since the loop may not wake again for a
// long time. A zero timeout means the wait is indefinite.
func (d *gLDriver) traceBeginWait(timeout time.Duration) traceWait {
	if !traceEnabled {
		return traceWait{}
	}

	wait := traceWait{started: time.Now(), stale: d.hasPendingMouseUpdate()}
	if wait.stale {
		tracef("sleeping with a pending mouse update (stale waits: %d, timeout: %s)"+
			" - hover and cursor stay stale until the next event",
			traceStaleWaits.Add(1), traceTimeout(timeout))
	}
	return wait
}

// traceEndWait reports how long the run loop was blocked. Ordinary short
// sleeps are not logged, or every mouse move would produce a line.
func (d *gLDriver) traceEndWait(wait traceWait) {
	if !traceEnabled || wait.started.IsZero() {
		return
	}

	slept := time.Since(wait.started)
	if wait.stale && slept >= traceLongWait {
		tracef("STALLED for %s with a mouse update still pending"+
			" (stalls: %d, wakes posted: %d) - this is the stuck cursor and dead hover case",
			slept.Round(time.Millisecond), traceStaleStalls.Add(1), traceWakes.Load())
		return
	}

	if !wait.stale && slept < traceLongWait {
		return
	}

	tracef("woke after %s (stale: %t, wakes posted: %d)",
		slept.Round(time.Microsecond), wait.stale, traceWakes.Load())
}

// traceWakePosted counts a request for the loop to stop sleeping.
func traceWakePosted() {
	if traceEnabled {
		traceWakes.Add(1)
	}
}

// traceResize reports a resize reaching the driver. During an interactive
// resize these are the only sign of life on platforms where the OS modal loop
// owns the thread, so they give the timeline a drag can be read against.
func (w *window) traceResize(width, height int) {
	if !traceEnabled {
		return
	}

	tracef("resize to %dx%d (pending mouse update: %t)",
		width, height, !w.mousePosUpdateProcessed)
}

// hasPendingMouseUpdate reports whether any window has a mouse position that
// the run loop has not applied yet. Main thread only.
func (d *gLDriver) hasPendingMouseUpdate() bool {
	for _, win := range d.windowList() {
		if w := win.(*window); !w.mousePosUpdateProcessed {
			return true
		}
	}
	return false
}

func traceTimeout(timeout time.Duration) string {
	if timeout <= 0 {
		return "indefinite"
	}
	return timeout.Round(time.Microsecond).String()
}
