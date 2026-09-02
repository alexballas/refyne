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

// loopTrace is the tracer the driver reports to. It is configured once, at
// startup, and never replaced: tests build their own tracer rather than
// reconfiguring this one, which would race with a running event loop.
var loopTrace = newTracer(os.Getenv(traceEnvKey) != "", os.Stderr)

type tracer struct {
	enabled bool
	start   time.Time

	lock sync.Mutex
	out  io.Writer

	// staleWaits counts waits entered with a mouse update still pending.
	// This on its own is harmless and happens in normal use: an event is
	// usually already queued and the loop wakes again in microseconds.
	staleWaits atomic.Uint64

	// staleStalls counts the waits above that then blocked for longer than
	// traceLongWait, which is the case that is actually visible as a stuck
	// cursor and dead hover. A report that comes with a non-zero count here
	// is this bug; one that comes with a zero count is not.
	staleStalls atomic.Uint64

	// wakes counts wakeEventLoop calls, i.e. how often something asked the
	// loop to stop sleeping. Reported alongside the waits rather than logged
	// on its own, which would drown the log during animation.
	wakes atomic.Uint64
}

// traceWait is the state a run loop sleep began in.
type traceWait struct {
	started time.Time
	stale   bool
}

func newTracer(enabled bool, out io.Writer) *tracer {
	return &tracer{enabled: enabled, start: time.Now(), out: out}
}

// logf writes one timestamped line. It is safe to call from any goroutine; the
// lock only keeps lines from interleaving.
func (t *tracer) logf(format string, args ...any) {
	t.lock.Lock()
	defer t.lock.Unlock()

	fmt.Fprintf(t.out, "[loop %12s] %s\n",
		time.Since(t.start).Round(time.Microsecond), fmt.Sprintf(format, args...))
}

// beginWait records the state a run loop sleep is starting in and reports the
// stale case immediately, since the loop may not wake again for a long time. A
// zero timeout means the wait is indefinite.
func (t *tracer) beginWait(stale bool, timeout time.Duration) traceWait {
	if !t.enabled {
		return traceWait{}
	}

	if stale {
		t.logf("sleeping with a pending mouse update (stale waits: %d, timeout: %s)"+
			" - hover and cursor stay stale until the next event",
			t.staleWaits.Add(1), traceTimeout(timeout))
	}
	return traceWait{started: time.Now(), stale: stale}
}

// endWait reports how long the run loop was blocked. Ordinary short sleeps are
// not logged, or every mouse move would produce a line.
func (t *tracer) endWait(wait traceWait) {
	if !t.enabled || wait.started.IsZero() {
		return
	}

	slept := time.Since(wait.started)
	if wait.stale && slept >= traceLongWait {
		t.logf("STALLED for %s with a mouse update still pending"+
			" (stalls: %d, wakes posted: %d) - this is the stuck cursor and dead hover case",
			slept.Round(time.Millisecond), t.staleStalls.Add(1), t.wakes.Load())
		return
	}

	if !wait.stale && slept < traceLongWait {
		return
	}

	t.logf("woke after %s (stale: %t, wakes posted: %d)",
		slept.Round(time.Microsecond), wait.stale, t.wakes.Load())
}

// resize reports a resize reaching the driver. During an interactive resize
// these are the only sign of life on platforms where the OS modal loop owns
// the thread, so they give the timeline a drag can be read against.
func (t *tracer) resize(width, height int, pendingMouse bool) {
	if !t.enabled {
		return
	}

	t.logf("resize to %dx%d (pending mouse update: %t)", width, height, pendingMouse)
}

func (t *tracer) framebufferResize(width, height int, canvasWidth, canvasHeight, scale, textureScale float32) {
	if !t.enabled {
		return
	}

	t.logf("framebuffer resize to %dx%d (canvas: %.0fx%.0f, scale: %.2f, texture scale: %.2f)",
		width, height, canvasWidth, canvasHeight, scale, textureScale)
}

func (t *tracer) framebufferSettled() {
	if t.enabled {
		t.logf("retired pre-resize EGL buffer; presenting repainted buffer")
	}
}

// wakePosted counts a request for the loop to stop sleeping.
func (t *tracer) wakePosted() {
	if t.enabled {
		t.wakes.Add(1)
	}
}

// The driver side of the tracer. Each of these repeats the enabled check so
// that tracing costs a single bool test when it is off, rather than the work
// of gathering what would be reported.

func (d *gLDriver) traceBeginWait(timeout time.Duration) traceWait {
	if !loopTrace.enabled {
		return traceWait{}
	}

	return loopTrace.beginWait(d.hasPendingMouseUpdate(), timeout)
}

func (d *gLDriver) traceEndWait(wait traceWait) {
	if !loopTrace.enabled {
		return
	}

	loopTrace.endWait(wait)
}

func (w *window) traceResize(width, height int) {
	if !loopTrace.enabled {
		return
	}

	loopTrace.resize(width, height, !w.mousePosUpdateProcessed)
}

func (w *window) traceFramebufferResize(width, height int) {
	if !loopTrace.enabled {
		return
	}

	size := w.canvas.Size()
	loopTrace.framebufferResize(width, height, size.Width, size.Height, w.canvas.scale, w.canvas.texScale)
}

func (w *window) traceFramebufferSettled() {
	if loopTrace.enabled {
		loopTrace.framebufferSettled()
	}
}

func traceWakePosted() {
	if loopTrace.enabled {
		loopTrace.wakePosted()
	}
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
