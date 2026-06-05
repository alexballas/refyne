// Package glfw provides a full Fyne desktop driver that uses the system OpenGL libraries.
// This supports Windows, Mac OS X and Linux using the gl and glfw packages from go-gl.
package glfw

import (
	"bytes"
	"image"
	"os"
	"runtime"

	"github.com/alexballas/refyne/v2/internal/async"
	"github.com/fyne-io/image/ico"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/animation"
	intapp "github.com/alexballas/refyne/v2/internal/app"
	"github.com/alexballas/refyne/v2/internal/driver"
	"github.com/alexballas/refyne/v2/internal/driver/common"
	"github.com/alexballas/refyne/v2/internal/painter"
	intRepo "github.com/alexballas/refyne/v2/internal/repository"
	"github.com/alexballas/refyne/v2/storage/repository"
)

var curWindow *window

// waylandRuntime records whether GLFW selected the Wayland backend at runtime.
// It is set once in initGLFW (desktop) after glfw.Init() and read via
// runningWayland(). Unlike the compile-time build.IsWayland constant it is also
// correct in the default Linux build, where both the X11 and Wayland backends
// are compiled in and the platform is only known at runtime. It stays false on
// non-desktop builds (wasm/test_web_driver) and on X11.
var waylandRuntime bool

// runningWayland reports whether the active GLFW platform is Wayland. Use this
// for any window behaviour that differs between X11 and Wayland (positioning,
// focus, scaling, custom decorations) so the default both-backends build does
// the right thing on whichever platform it ends up running.
func runningWayland() bool { return waylandRuntime }

// Declare conformity with Driver
var _ fyne.Driver = (*gLDriver)(nil)

type gLDriver struct {
	windows     []fyne.Window
	initialized bool
	done        chan struct{}

	animation animation.Runner

	currentKeyModifiers fyne.KeyModifier // desktop driver only

	trayStart, trayStop func()     // shut down the system tray, if used
	systrayMenu         *fyne.Menu // cache the menu set so we know when to refresh
}

func (d *gLDriver) init() {
	if !d.initialized {
		d.initialized = true
		d.initGLFW()
	}
}

func toOSIcon(icon []byte) ([]byte, error) {
	if runtime.GOOS != "windows" {
		return icon, nil
	}

	img, _, err := image.Decode(bytes.NewReader(icon))
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	err = ico.Encode(buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (d *gLDriver) DoFromGoroutine(f func(), wait bool) {
	if wait {
		async.EnsureNotMain(func() {
			runOnMainWithWait(f, true)
		})
	} else {
		runOnMainWithWait(f, false)
	}
}

func (d *gLDriver) RenderedTextSize(text string, textSize float32, style fyne.TextStyle, source fyne.Resource) (size fyne.Size, baseline float32) {
	return painter.RenderedTextSize(text, textSize, style, source)
}

func (d *gLDriver) CanvasForObject(obj fyne.CanvasObject) fyne.Canvas {
	return common.CanvasForObject(obj)
}

func (d *gLDriver) AbsolutePositionForObject(co fyne.CanvasObject) fyne.Position {
	c := d.CanvasForObject(co)
	if c == nil {
		return fyne.NewPos(0, 0)
	}

	glc := c.(*glCanvas)
	pos := driver.AbsolutePositionForObject(co, glc.ObjectTrees())
	inset, _ := glc.InteractiveArea()
	return pos.Subtract(inset)
}

func (d *gLDriver) Device() fyne.Device {
	return &glDevice{}
}

func (d *gLDriver) Quit() {
	if curWindow != nil {
		if f := fyne.CurrentApp().Lifecycle().(*intapp.Lifecycle).OnExitedForeground(); f != nil {
			f()
		}
		curWindow = nil
		if d.trayStop != nil {
			d.trayStop()
		}
	}

	// Only call close once to avoid panic.
	if running.CompareAndSwap(true, false) {
		close(d.done)
	}
}

func (d *gLDriver) addWindow(w *window) {
	d.windows = append(d.windows, w)
}

// a trivial implementation of "focus previous" - return to the most recently opened, or master if set.
// This may not do the right thing if your app has 3 or more windows open, but it was agreed this was not much
// of an issue, and the added complexity to track focus was not needed at this time.
func (d *gLDriver) focusPreviousWindow() {
	var chosen *window
	for _, w := range d.windows {
		win := w.(*window)
		if !win.visible {
			continue
		}
		chosen = win
		if win.master {
			break
		}
	}

	if chosen == nil || chosen.view() == nil {
		return
	}
	chosen.RequestFocus()
}

func (d *gLDriver) windowList() []fyne.Window {
	return d.windows
}

func (d *gLDriver) initFailed(msg string, err error) {
	fyne.LogError(msg, err)

	if !running.Load() {
		d.Quit()
	} else {
		os.Exit(1)
	}
}

func (d *gLDriver) Run() {
	if !async.IsMainGoroutine() {
		panic("Run() or ShowAndRun() must be called from main goroutine")
	}

	go d.catchTerm()
	d.runGL()

	// Ensure lifecycle events run to completion before the app exits
	l := fyne.CurrentApp().Lifecycle().(*intapp.Lifecycle)
	l.WaitForEvents()
	l.DestroyEventQueue()
}

func (d *gLDriver) SetDisableScreenBlanking(disable bool) {
	setDisableScreenBlank(disable)
}

// NewGLDriver sets up a new Driver instance implemented using the GLFW Go library and OpenGL bindings.
func NewGLDriver() *gLDriver {
	repository.Register("file", intRepo.NewFileRepository())

	return &gLDriver{
		done: make(chan struct{}),
	}
}
