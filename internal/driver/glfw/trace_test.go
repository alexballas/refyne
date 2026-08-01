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

// enableTrace turns tracing on with output captured, restoring both when the
// test ends.
func enableTrace(t *testing.T) *bytes.Buffer {
	t.Helper()

	out := &bytes.Buffer{}
	oldEnabled, oldOutput := traceEnabled, traceOutput
	traceEnabled, traceOutput = true, out
	t.Cleanup(func() {
		traceEnabled, traceOutput = oldEnabled, oldOutput
	})

	return out
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
	out := enableTrace(t)
	traceEnabled = false

	d := &gLDriver{windows: []fyne.Window{traceTestWindow(true)}}
	before := traceStaleWaits.Load()

	wait := d.traceBeginWait(0)
	d.traceEndWait(wait)

	if !wait.started.IsZero() {
		t.Error("disabled tracing recorded a wait")
	}
	if got := traceStaleWaits.Load(); got != before {
		t.Errorf("disabled tracing counted a stale wait: %d, want %d", got, before)
	}
	if out.Len() != 0 {
		t.Errorf("disabled tracing wrote output: %q", out.String())
	}
}

func TestTraceReportsStaleWait(t *testing.T) {
	out := enableTrace(t)

	d := &gLDriver{windows: []fyne.Window{traceTestWindow(true)}}
	before := traceStaleWaits.Load()

	wait := d.traceBeginWait(0)
	if !wait.stale {
		t.Fatal("pending mouse update was not detected")
	}
	if got := traceStaleWaits.Load(); got != before+1 {
		t.Errorf("stale wait count = %d, want %d", got, before+1)
	}
	if logged := out.String(); !strings.Contains(logged, "pending mouse update") ||
		!strings.Contains(logged, "indefinite") {
		t.Errorf("unexpected stale wait line: %q", logged)
	}

	out.Reset()
	d.traceEndWait(wait)
	if logged := out.String(); !strings.Contains(logged, "woke after") {
		t.Errorf("stale wait did not report waking: %q", logged)
	}
}

func TestTraceIgnoresHealthyShortWait(t *testing.T) {
	out := enableTrace(t)

	d := &gLDriver{windows: []fyne.Window{traceTestWindow(false)}}

	wait := d.traceBeginWait(time.Second / 60)
	if wait.stale {
		t.Fatal("window with no pending mouse update reported as stale")
	}

	d.traceEndWait(wait)
	if out.Len() != 0 {
		t.Errorf("healthy short wait wrote output: %q", out.String())
	}
}

func TestTraceReportsStall(t *testing.T) {
	out := enableTrace(t)

	d := &gLDriver{windows: []fyne.Window{traceTestWindow(true)}}
	before := traceStaleStalls.Load()

	wait := d.traceBeginWait(0)
	out.Reset()
	wait.started = wait.started.Add(-2 * traceLongWait) // pretend we slept
	d.traceEndWait(wait)

	if got := traceStaleStalls.Load(); got != before+1 {
		t.Errorf("stall count = %d, want %d", got, before+1)
	}
	if logged := out.String(); !strings.Contains(logged, "STALLED") {
		t.Errorf("stalled wait was not reported: %q", logged)
	}
}

func TestTraceReportsLongWait(t *testing.T) {
	out := enableTrace(t)

	d := &gLDriver{windows: []fyne.Window{traceTestWindow(false)}}

	wait := d.traceBeginWait(0)
	wait.started = wait.started.Add(-2 * traceLongWait) // pretend we slept
	d.traceEndWait(wait)

	if logged := out.String(); !strings.Contains(logged, "woke after") {
		t.Errorf("long wait was not reported: %q", logged)
	}
}

func TestTraceResize(t *testing.T) {
	out := enableTrace(t)

	traceTestWindow(true).traceResize(320, 240)

	logged := out.String()
	if !strings.Contains(logged, "320x240") {
		t.Errorf("resize size missing from trace: %q", logged)
	}
	if !strings.Contains(logged, "pending mouse update: true") {
		t.Errorf("resize did not report the pending mouse update: %q", logged)
	}
}

func TestTraceWakePostedCounts(t *testing.T) {
	enableTrace(t)

	before := traceWakes.Load()
	traceWakePosted()
	if got := traceWakes.Load(); got != before+1 {
		t.Errorf("wake count = %d, want %d", got, before+1)
	}

	traceEnabled = false
	traceWakePosted()
	if got := traceWakes.Load(); got != before+1 {
		t.Errorf("disabled tracing counted a wake: %d, want %d", got, before+1)
	}
}
