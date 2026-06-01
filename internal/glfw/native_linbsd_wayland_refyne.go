//go:build (linux && wayland) || (freebsd && wayland) || (netbsd && wayland) || (openbsd && wayland)
// +build linux,wayland freebsd,wayland netbsd,wayland openbsd,wayland

package glfw

// Go wrappers for the refyne Wayland decoration helpers defined in
// glfw/src/wl_refyne.c. See ../VENDORING.md — this is a local patch over
// go-gl/glfw, kept in its own file so upstream re-syncs stay mechanical.

//#define GLFW_INCLUDE_NONE
//#include "glfw/include/GLFW/glfw3.h"
//extern void glfwRefyneStartWindowMove(GLFWwindow* handle);
//extern void glfwRefyneStartWindowResize(GLFWwindow* handle, int edges);
//extern void glfwRefyneShowWindowMenu(GLFWwindow* handle, int xpos, int ypos);
//extern int  glfwRefyneDecorationMode(GLFWwindow* handle);
//extern int  glfwRefyneSetWindowIcon(GLFWwindow* handle, const unsigned char* pixels, int width, int height);
import "C"

import "unsafe"

// ResizeEdge identifies which window edge or corner an interactive resize
// grabs. Values match xdg_toplevel.resize_edge.
type ResizeEdge int

const (
	ResizeEdgeNone        ResizeEdge = 0
	ResizeEdgeTop         ResizeEdge = 1
	ResizeEdgeBottom      ResizeEdge = 2
	ResizeEdgeLeft        ResizeEdge = 4
	ResizeEdgeTopLeft     ResizeEdge = 5
	ResizeEdgeBottomLeft  ResizeEdge = 6
	ResizeEdgeRight       ResizeEdge = 8
	ResizeEdgeTopRight    ResizeEdge = 9
	ResizeEdgeBottomRight ResizeEdge = 10
)

// DecorationMode reports the xdg-decoration mode a Wayland compositor granted a
// window. Mirrors zxdg_toplevel_decoration_v1's mode values.
type DecorationMode int

const (
	// DecorationModeNone means the compositor never granted a mode (e.g.
	// GNOME/Mutter, which does not implement xdg-decoration) — draw our own.
	DecorationModeNone DecorationMode = 0
	// DecorationModeClientSide means the client must draw decorations.
	DecorationModeClientSide DecorationMode = 1
	// DecorationModeServerSide means the compositor draws decorations (SSD).
	DecorationModeServerSide DecorationMode = 2
)

// StartWindowMove asks the compositor to begin an interactive move of the
// window, reusing GLFW's already-tracked seat and most recent input serial.
// Call it from a pointer-button handler (the serial must be fresh). Wayland only.
func (w *Window) StartWindowMove() {
	C.glfwRefyneStartWindowMove(w.data)
	panicError()
}

// StartWindowResize asks the compositor to begin an interactive edge/corner
// resize of the window, reusing GLFW's tracked seat and input serial. Wayland only.
func (w *Window) StartWindowResize(edge ResizeEdge) {
	C.glfwRefyneStartWindowResize(w.data, C.int(edge))
	panicError()
}

// ShowWindowMenu asks the compositor to show its standard window menu at the
// given surface-local coordinates, reusing GLFW's tracked seat and input serial.
// Call it directly from a user-action handler so the serial is still fresh.
// Wayland only.
func (w *Window) ShowWindowMenu(xpos, ypos int) {
	C.glfwRefyneShowWindowMenu(w.data, C.int(xpos), C.int(ypos))
	panicError()
}

// DecorationMode returns the xdg-decoration mode granted to the window. Use it
// to decide whether refyne must draw client-side decorations (None/ClientSide)
// or can rely on the compositor (ServerSide). Wayland only.
func (w *Window) DecorationMode() DecorationMode {
	mode := C.glfwRefyneDecorationMode(w.data)
	panicError()
	return DecorationMode(mode)
}

// SetWindowIconWayland pushes an application icon to the compositor via the
// xdg-toplevel-icon-v1 protocol. pixels must be tightly-packed, non-premultiplied
// RGBA8888 of length width*height*4, and the icon must be square (width==height)
// — the protocol rejects non-square buffers. Returns false when the compositor
// does not support the protocol (e.g. GNOME) or on invalid input. Wayland only;
// call on the main thread.
func (w *Window) SetWindowIconWayland(pixels []byte, width, height int) bool {
	if width <= 0 || height <= 0 || len(pixels) < width*height*4 {
		return false
	}
	res := C.glfwRefyneSetWindowIcon(w.data,
		(*C.uchar)(unsafe.Pointer(&pixels[0])), C.int(width), C.int(height))
	panicError()
	return res != 0
}
