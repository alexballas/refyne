package glfw

import (
	"runtime"
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

// channel for queuing functions on the main thread
var (
	funcQueue        = async.NewUnboundedChan[funcData]()
	running, drained atomic.Bool
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

		funcQueue.In() <- funcData{f: f, done: done}
		<-done
	} else {
		funcQueue.In() <- funcData{f: f}
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

	// Drive the loop at the display's refresh rate rather than a fixed 60 Hz so
	// resizing and animation stay smooth on high-refresh monitors. rateTick
	// re-evaluates this once a second to pick up monitor hotplug / mode changes
	// (e.g. docking a laptop to a 144 Hz screen).
	frameInterval := d.frameInterval()
	eventTick := time.NewTicker(frameInterval)
	rateTick := time.NewTicker(time.Second)
	for {
		select {
		case <-d.done:
			eventTick.Stop()
			rateTick.Stop()
			d.Terminate()
			l := fyne.CurrentApp().Lifecycle().(*app.Lifecycle)
			if f := l.OnStopped(); f != nil {
				l.QueueEvent(f)
			}

			// as we are shutting down make sure we drain the pending funcQueue and close it out.
			for len(funcQueue.Out()) > 0 {
				f := <-funcQueue.Out()
				if f.done != nil {
					f.done <- struct{}{}
				}
			}
			drained.Store(true)
			funcQueue.Close()
			return
		case f := <-funcQueue.Out():
			f.f()
			if f.done != nil {
				f.done <- struct{}{}
			}
		case <-rateTick.C:
			if interval := d.frameInterval(); interval != frameInterval {
				frameInterval = interval
				eventTick.Reset(interval)
			}
		case <-eventTick.C:
			d.pollEvents()
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

				if expand && !fullScreen {
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

			d.animation.TickAnimations()
			d.drawSingleFrame()
		}
	}
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
