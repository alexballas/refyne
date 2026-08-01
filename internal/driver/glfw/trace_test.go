//go:build !no_glfw && !mobile

package glfw

import (
	"bytes"
	"strings"
	"testing"
	"time"

	fyne "github.com/alexballas/refyne/v2"
	iglfw "github.com/alexballas/refyne/v2/internal/glfw"
)

// newTestTracer returns a tracer writing to a buffer the caller owns. Tests
// never touch loopTrace: the package runs a real event loop for the duration
// of the test binary and would race with, and write into, anything shared.
func newTestTracer(enabled bool) (*tracer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return newTracer(enabled, out), out
}

func traceTestWindow(mousePending bool) *window {
	return &window{
		canvas:                  newCanvas(),
		frame:                   noGate{},
		viewport:                &iglfw.Window{},
		visible:                 true,
		mousePosUpdateProcessed: !mousePending,
	}
}

func TestTraceDisabledIsSilent(t *testing.T) {
	tr, out := newTestTracer(false)

	wait := tr.beginWait(true, 0)
	tr.endWait(wait)
	tr.resize(320, 240, true)
	tr.wakePosted()

	if !wait.started.IsZero() {
		t.Error("disabled tracing recorded a wait")
	}
	if got := tr.staleWaits.Load(); got != 0 {
		t.Errorf("disabled tracing counted %d stale waits, want 0", got)
	}
	if got := tr.wakes.Load(); got != 0 {
		t.Errorf("disabled tracing counted %d wakes, want 0", got)
	}
	if out.Len() != 0 {
		t.Errorf("disabled tracing wrote output: %q", out.String())
	}
}

func TestTraceReportsStaleWait(t *testing.T) {
	tr, out := newTestTracer(true)

	wait := tr.beginWait(true, 0)
	if !wait.stale {
		t.Fatal("stale wait was not recorded as stale")
	}
	if got := tr.staleWaits.Load(); got != 1 {
		t.Errorf("stale wait count = %d, want 1", got)
	}
	if logged := out.String(); !strings.Contains(logged, "pending mouse update") ||
		!strings.Contains(logged, "indefinite") {
		t.Errorf("unexpected stale wait line: %q", logged)
	}

	out.Reset()
	tr.endWait(wait)
	if logged := out.String(); !strings.Contains(logged, "woke after") {
		t.Errorf("stale wait did not report waking: %q", logged)
	}
	if got := tr.staleStalls.Load(); got != 0 {
		t.Errorf("a short stale wait was counted as a stall: %d", got)
	}
}

func TestTraceIgnoresHealthyShortWait(t *testing.T) {
	tr, out := newTestTracer(true)

	wait := tr.beginWait(false, time.Second/60)
	tr.endWait(wait)

	if out.Len() != 0 {
		t.Errorf("healthy short wait wrote output: %q", out.String())
	}
}

func TestTraceReportsStall(t *testing.T) {
	tr, out := newTestTracer(true)

	wait := tr.beginWait(true, 0)
	out.Reset()
	wait.started = wait.started.Add(-2 * traceLongWait) // pretend we slept
	tr.endWait(wait)

	if got := tr.staleStalls.Load(); got != 1 {
		t.Errorf("stall count = %d, want 1", got)
	}
	if logged := out.String(); !strings.Contains(logged, "STALLED") {
		t.Errorf("stalled wait was not reported: %q", logged)
	}
}

func TestTraceReportsLongWait(t *testing.T) {
	tr, out := newTestTracer(true)

	wait := tr.beginWait(false, 0)
	wait.started = wait.started.Add(-2 * traceLongWait) // pretend we slept
	tr.endWait(wait)

	if logged := out.String(); !strings.Contains(logged, "woke after") {
		t.Errorf("long wait was not reported: %q", logged)
	}
	if got := tr.staleStalls.Load(); got != 0 {
		t.Errorf("a healthy wait was counted as a stall: %d", got)
	}
}

func TestTraceResize(t *testing.T) {
	tr, out := newTestTracer(true)

	tr.resize(320, 240, true)

	logged := out.String()
	if !strings.Contains(logged, "320x240") {
		t.Errorf("resize size missing from trace: %q", logged)
	}
	if !strings.Contains(logged, "pending mouse update: true") {
		t.Errorf("resize did not report the pending mouse update: %q", logged)
	}
}

func TestTraceWakePostedCounts(t *testing.T) {
	tr, out := newTestTracer(true)

	tr.wakePosted()
	tr.wakePosted()

	if got := tr.wakes.Load(); got != 2 {
		t.Errorf("wake count = %d, want 2", got)
	}
	if out.Len() != 0 {
		t.Errorf("wake counting wrote output: %q", out.String())
	}
}

func TestHasPendingMouseUpdate(t *testing.T) {
	d := &gLDriver{windows: []fyne.Window{traceTestWindow(false)}}
	if d.hasPendingMouseUpdate() {
		t.Error("driver with no pending mouse update reported one")
	}

	d.windows = append(d.windows, traceTestWindow(true))
	if !d.hasPendingMouseUpdate() {
		t.Error("pending mouse update was not detected")
	}
}
