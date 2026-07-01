package glfw

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/app"
	"github.com/alexballas/refyne/v2/internal/async"
	"github.com/alexballas/refyne/v2/internal/cache"
	"github.com/alexballas/refyne/v2/internal/driver/common"
	"github.com/alexballas/refyne/v2/internal/painter"
	"github.com/alexballas/refyne/v2/internal/scale"
)

type funcData struct {
	f    func()
	done chan struct{} // Zero allocation signalling channel
}

// glfwFuncQueue is a synchronous queue of functions to run on the main thread.
// Unlike an unbounded channel it has no shuttle goroutine, so a pushed func is
// visible to the draining main thread the instant push returns. That removes
// the window where the main thread could observe an empty queue while work is
// still in flight (and then sleep through it), so we can wake the event loop on
// just the empty→non-empty transition rather than on every enqueue.
type glfwFuncQueue struct {
	mu      sync.Mutex
	pending []funcData
}

// push appends f and wakes the event loop if the queue was previously empty.
func (q *glfwFuncQueue) push(f funcData) {
	q.mu.Lock()
	wasEmpty := len(q.pending) == 0
	q.pending = append(q.pending, f)
	q.mu.Unlock()

	if wasEmpty {
		wakeEventLoop()
	}
}

// drain hands the caller every pending func and resets the queue. It returns
// nil without allocating when the queue is empty.
func (q *glfwFuncQueue) drain() []funcData {
	q.mu.Lock()
	funcs := q.pending
	q.pending = nil
	q.mu.Unlock()
	return funcs
}

// queue of functions to run on the main thread
var (
	funcQueue      = &glfwFuncQueue{}
	running        atomic.Bool
	drained        atomic.Bool
	eventLoopReady atomic.Bool
)

// Arrange that main.main runs on main thread.
func init() {
	runtime.LockOSThread()
	async.SetMainGoroutine()
}

// force a function f to run on the main thread
func runOnMain(f func()) {
	runOnMainWithWait(f, true)
}

// force a function f to run on the main thread and specify if we should wait for it to return
func runOnMainWithWait(f func(), wait bool) {
	// If we are on main before app run just execute - otherwise add it to the main queue and wait.
	// We also need to run it as-is if the app is in the process of shutting down as the queue will be stopped.
	if (!running.Load() && async.IsMainGoroutine()) || drained.Load() {
		f()
		return
	}

	if wait {
		done := common.DonePool.Get()
		defer common.DonePool.Put(done)

		queueMainFunc(funcData{f: f, done: done})
		<-done
	} else {
		queueMainFunc(funcData{f: f})
	}
}

func queueMainFunc(f funcData) {
	funcQueue.push(f)
}

func runQueuedFunc(f funcData) {
	f.f()
	if f.done != nil {
		f.done <- struct{}{}
	}
}

func finishQueuedFunc(f funcData) {
	if f.done != nil {
		f.done <- struct{}{}
	}
}

// decideRepaint reports whether a window should be repainted this frame.
// checkDirtyAndClear is only called when the window is visible and presentable,
// so a window that is not yet presentable keeps its dirty flag and the frame is
// deferred until the compositor is ready (see issue #6080).
func decideRepaint(visible, ready bool, checkDirtyAndClear func() bool) bool {
	return visible && ready && checkDirtyAndClear()
}

func (d *gLDriver) drawSingleFrame() {
	cache.BeginFrame()

	refreshed := false
	for _, win := range d.windowList() {
		w := win.(*window)
		if w.closing {
			continue
		}

		// Apply any coalesced interactive resize before deciding to repaint, so
		// a burst of configure events costs a single canvas.Resize per frame.
		w.applyPendingResize()

		// Repaint only when the window is visible AND the compositor is ready
		// to present it (Wayland frame callback). decideRepaint consults the
		// dirty flag last, so a window that is not yet presentable keeps its
		// dirty flag and the frame is deferred until the compositor is ready
		// (issue #6080). When we are not repainting, keep the render caches
		// alive instead.
		if decideRepaint(w.visible, w.frame.ready(), w.canvas.CheckDirtyAndClear) {
			w.RunWithContext(func() {
				if w.driver.repaintWindow(w) {
					refreshed = true
				}
			})
			w.updateAccessibility()
		} else {
			w.markCacheAlive()
		}
	}
	cache.Clean(refreshed)
}

// markCacheAlive keeps a non-drawn window's render caches from expiring without
// repainting it.
func (w *window) markCacheAlive() {
	threshold := time.Now().Add(10*time.Second - cache.ValidDuration)
	if w.lastWalkedTime.Before(threshold) {
		w.canvas.WalkTrees(nil, func(node *common.RenderCacheNode, _ fyne.Position) {
			// marks canvas for object cache entry alive
			_ = cache.GetCanvasForObject(node.Obj())
			// marks renderer cache entry alive
			if wid, ok := node.Obj().(fyne.Widget); ok {
				_, _ = cache.CachedRenderer(wid)
			}
		})
		w.lastWalkedTime = time.Now()
	}
}

func (d *gLDriver) runGL() {
	if !running.CompareAndSwap(false, true) {
		return // Run was called twice.
	}

	d.init()
	eventLoopReady.Store(true)
	if d.trayStart != nil {
		d.trayStart()
	}

	fyne.CurrentApp().Settings().AddListener(func(set fyne.Settings) {
		painter.ClearFontCache()
		cache.ResetThemeCaches()
		app.ApplySettingsWithCallback(set, fyne.CurrentApp(), func(w fyne.Window) {
			c, ok := w.Canvas().(*glCanvas)
			if !ok {
				return
			}
			c.applyThemeOutOfTreeObjects()
			c.reloadScale()
		})
	})

	if f := fyne.CurrentApp().Lifecycle().(*app.Lifecycle).OnStarted(); f != nil {
		f()
	}

	// Drive active work at the display's refresh rate rather than a fixed 60 Hz
	// so resizing and animation stay smooth on high-refresh monitors. When no
	// animations, dirty canvases or queued functions exist, block indefinitely in
	// the native event wait instead of ticking at refresh rate forever.
	frameInterval := d.frameInterval()
	nextRateCheck := time.Now().Add(time.Second)
	nextFrame := time.Now()
	for {
		if d.processQueuedFuncsOrDone() {
			return
		}

		now := time.Now()
		if !now.Before(nextRateCheck) {
			frameInterval = d.frameInterval()
			nextRateCheck = now.Add(time.Second)
		}

		if d.needsFrameTick() {
			if wait := time.Until(nextFrame); wait > 0 {
				d.waitEventsTimeout(wait)
				if d.processQueuedFuncsOrDone() {
					return
				}
				d.processWindowEvents()
				continue
			}
			d.runFrame()
			// Advance the deadline by a whole interval from the previous
			// deadline (not from time.Now()) so draw time and timer-wakeup
			// overshoot are absorbed by the absolute schedule rather than
			// accumulating into a slower-than-target frame rate. If a slow
			// frame has put us more than one interval behind, resync to now
			// so we don't issue a burst of catch-up frames.
			nextFrame = nextFrame.Add(frameInterval)
			if now := time.Now(); nextFrame.Before(now) {
				nextFrame = now
			}
			continue
		}

		d.waitEvents()
		if d.processQueuedFuncsOrDone() {
			return
		}
		d.processWindowEvents()
		if d.needsFrameTick() {
			d.runFrame()
			nextFrame = time.Now().Add(frameInterval)
		}
	}
}

func (d *gLDriver) processQueuedFuncsOrDone() bool {
	for {
		select {
		case <-d.done:
			d.shutdownGL()
			return true
		default:
		}

		funcs := funcQueue.drain()
		if len(funcs) == 0 {
			return false
		}
		for i := range funcs {
			runQueuedFunc(funcs[i])
		}
	}
}

func (d *gLDriver) shutdownGL() {
	eventLoopReady.Store(false)
	d.Terminate()
	l := fyne.CurrentApp().Lifecycle().(*app.Lifecycle)
	if f := l.OnStopped(); f != nil {
		l.QueueEvent(f)
	}

	// Stop accepting new queued funcs (runOnMainWithWait now runs them inline)
	// and finish any that are already pending so their callers stop waiting.
	// Looping covers funcs pushed by goroutines that passed the drained check
	// just before the flag flipped; once drained is set no new ones can arrive,
	// so the in-flight set is finite.
	drained.Store(true)
	for {
		funcs := funcQueue.drain()
		if len(funcs) == 0 {
			return
		}
		for i := range funcs {
			finishQueuedFunc(funcs[i])
		}
	}
}

func (d *gLDriver) runFrame() {
	d.processWindowEvents()
	d.animation.TickAnimations()
	d.drawSingleFrame()
}

func (d *gLDriver) processWindowEvents() {
	for i := 0; i < len(d.windows); i++ {
		w := d.windows[i].(*window)
		if !w.mousePosUpdateProcessed {
			w.processMouseMoved(w.newMousePosX, w.newMousePosY)
			w.mousePosUpdateProcessed = true
		}

		if w.viewport == nil {
			continue
		}

		if w.viewport.ShouldClose() {
			d.destroyWindow(w, i)
			i-- // Trailing windows are moved forward one step.
			continue
		}

		expand := w.shouldExpand
		fullScreen := w.fullScreen

		// While the compositor drives an interactive resize the client must
		// follow its configure sizes. Pushing back with SetSize (e.g. when the
		// drag goes below the content minimum) makes the two sides commit
		// alternating sizes every frame — visible as the window trembling.
		// shouldExpand stays set, so the min size is enforced when the grab ends.
		if expand && !fullScreen && !w.interactiveResizing() {
			w.fitContent()
			shouldExpand := w.shouldExpand
			w.shouldExpand = false
			view := w.viewport

			if shouldExpand && runtime.GOOS != "js" {
				view.SetSize(w.shouldWidth, w.shouldHeight)
				// On Wayland a client-initiated resize fires no size
				// callback and no compositor configure echo, so apply
				// the new size to the canvas directly (as Resize does)
				// or w.width/height and the canvas would go stale.
				w.processResized(w.shouldWidth, w.shouldHeight)
			}
		}
	}
}

func (d *gLDriver) needsFrameTick() bool {
	return d.animation.Running() || d.hasReadyDirtyWindow()
}

func (d *gLDriver) hasReadyDirtyWindow() bool {
	for _, win := range d.windowList() {
		w := win.(*window)
		if w.closing || !w.visible || w.viewport == nil {
			continue
		}
		if w.frame.ready() && w.canvas.Dirty() {
			return true
		}
	}
	return false
}

func (d *gLDriver) destroyWindow(w *window, index int) {
	w.visible = false
	w.viewport.Destroy()
	w.destroy(d)

	if index < len(d.windows)-1 {
		copy(d.windows[index:], d.windows[index+1:])
	}
	d.windows[len(d.windows)-1] = nil
	d.windows = d.windows[:len(d.windows)-1]

	if len(d.windows) == 0 {
		d.Quit()
	}
}

func (d *gLDriver) repaintWindow(w *window) bool {
	canvas := w.canvas
	freed := false
	if canvas.EnsureMinSize() {
		w.shouldExpand = true
	}
	freed = canvas.FreeDirtyTextures() > 0

	updateGLContext(w)
	canvas.paint(canvas.Size())

	view := w.viewport
	visible := w.visible

	if view != nil && visible {
		// Request a frame callback for the surface; the SwapBuffers commit
		// below delivers the request. No-op off Wayland. After this, the gate
		// reports not-ready until the compositor presents us again, so we will
		// not issue another (potentially blocking) swap on a suspended surface.
		w.frame.arm(windowSurface(w))
		view.SwapBuffers()

	}

	// mark that we have walked the window and don't
	// need to walk it again to mark caches alive
	w.lastWalkedTime = time.Now()
	return freed
}

// refreshWindow requests that the specified window be redrawn
func refreshWindow(w *window) {
	w.canvas.SetDirty()
}

func updateGLContext(w *window) {
	canvas := w.canvas
	size := canvas.Size()

	// w.width and w.height are not correct if we are maximised, so figure from canvas
	winWidth := float32(scale.ToScreenCoordinate(canvas, size.Width)) * canvas.texScale
	winHeight := float32(scale.ToScreenCoordinate(canvas, size.Height)) * canvas.texScale

	canvas.Painter().SetFrameBufferScale(canvas.texScale)
	canvas.Painter().SetOutputSize(int(winWidth), int(winHeight))
}
