//go:build !wasm && wayland && (linux || freebsd || openbsd || netbsd)

package glfw

/*
#cgo pkg-config: wayland-client
#include <stdlib.h>
#include <wayland-client.h>

// frame_state holds the presentable flag for one surface. It lives in C so no
// Go pointer is stored across the cgo boundary.
typedef struct { int ready; } frame_state;

static void frame_done(void *data, struct wl_callback *cb, uint32_t t) {
    (void)t;
    ((frame_state *)data)->ready = 1;   // compositor presented us
    wl_callback_destroy(cb);
}
static const struct wl_callback_listener frame_listener = { frame_done };

static frame_state *frame_state_new(void) {
    frame_state *s = calloc(1, sizeof(frame_state));
    s->ready = 1;                        // first frame may proceed
    return s;
}
// frame_arm requests a frame callback and marks the surface not-ready. No
// commit here: the eglSwapBuffers that follows carries the request.
static void frame_arm(struct wl_surface *surface, frame_state *s) {
    s->ready = 0;
    struct wl_callback *cb = wl_surface_frame(surface);
    wl_callback_add_listener(cb, &frame_listener, s);
}
static int  frame_ready(frame_state *s) { return s->ready; }
static void frame_state_free(frame_state *s) { free(s); }
*/
import "C"

import "unsafe"

type frameTracker struct{ state *C.frame_state }

func newPresentGate() presentGate { return &frameTracker{state: C.frame_state_new()} }

func (t *frameTracker) ready() bool { return C.frame_ready(t.state) != 0 }

func (t *frameTracker) arm(surface unsafe.Pointer) {
	if surface == nil {
		return
	}
	C.frame_arm((*C.struct_wl_surface)(surface), t.state)
}

func (t *frameTracker) markReady() { t.state.ready = 1 }

func (t *frameTracker) free() { C.frame_state_free(t.state) }

// windowSurface returns the window's *wl_surface as an opaque pointer.
func windowSurface(w *window) unsafe.Pointer {
	if w.viewport == nil {
		return nil
	}
	return unsafe.Pointer(w.viewport.GetWaylandWindow())
}
