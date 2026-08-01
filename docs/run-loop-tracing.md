# Run loop tracing (`FYNE_TRACE_LOOP`)

Diagnostic tracing for the desktop GLFW run loop, aimed at reports of a window
that stops responding to the mouse after an interactive resize: the resize
cursor keeps showing, hover stops working, and the first click afterwards goes
somewhere unexpected.

Set the variable to any non-empty value and run the application as usual — no
rebuild is needed, so a bug reporter can use the binary they already have:

```bash
FYNE_TRACE_LOOP=1 ./myapp
```

Trace lines are written to stderr, prefixed with the time since startup.

## Why it exists

Mouse positions are recorded in the GLFW cursor callback and applied later, from
`processWindowEvents` on the next run loop iteration. Since the loop blocks in
`glfw.WaitEvents` when idle, a mouse update can be left pending while the loop
sleeps, which leaves hover state and the cursor stale until the next event
arrives.

That gap is normally microseconds. It matters on macOS and Windows, where the OS
runs a modal loop for the whole of an interactive resize and the run loop cannot
iterate at all until the drag ends. The trace exists to say whether a given
report is that problem or something else.

## Reading the output

```
[loop    139.934ms] resize to 71x76 (pending mouse update: true)
[loop    146.293ms] sleeping with a pending mouse update (stale waits: 1, timeout: indefinite) - ...
[loop    146.311ms] woke after 18µs (stale: true, wakes posted: 0)
[loop     3.04513s] woke after 2.879235s (stale: false, wakes posted: 3)
```

* `resize to WxH` — a resize reached the driver. During an interactive resize on
  macOS and Windows these are the only sign of life while the OS owns the
  thread, so they give a drag a timeline.
* `sleeping with a pending mouse update` — the loop blocked with mouse state it
  had not applied yet. **On its own this is harmless and happens in normal use**;
  an event is usually already queued and the loop wakes again in microseconds.
* `woke after ...` — how long the loop was blocked. Healthy short sleeps are not
  logged, or every mouse move would produce a line.
* `STALLED for ... with a mouse update still pending` — the case that is
  actually visible to a user. The loop slept for more than 250 ms holding stale
  mouse state, so the cursor and hover were wrong for that whole period.
* `wakes posted` — how many times something asked the loop to stop sleeping
  (`glfw.PostEmptyEvent`). Useful for telling "nothing asked the loop to wake"
  apart from "something did and it woke anyway".

## What to collect for a report

1. The trace log, with the moment the UI misbehaved noted against the
   timestamps.
2. Whether `STALLED` appears at that moment. If it does not, the run loop was
   awake and the cause is elsewhere.
3. Whether moving the mouse without clicking clears the problem. Together with
   the log this separates a sleeping run loop from corrupted platform cursor
   state.
4. Operating system and version, and a screen recording if the wording is hard
   to pin down.

## Cost

The environment variable is read once at startup, so tracing costs a single
bool test per wait when it is off — no timing calls, no allocation, no output.
When it is on, lines are written to stderr under a lock, including from the
goroutine that posts wake-ups, so a traced run is not a perfectly clean sample
of untraced timing.
