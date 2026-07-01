//go:build !wasm && !test_web_driver && ((linux && (wayland || !x11)) || ((freebsd || netbsd || openbsd) && wayland))

package glfw

import (
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	nativeglfw "github.com/alexballas/refyne/v2/internal/glfw"
	"github.com/stretchr/testify/assert"
)

func TestWaylandResizeEdgeAt(t *testing.T) {
	size := fyne.NewSize(100, 80)
	tests := []struct {
		name string
		pos  fyne.Position
		want nativeglfw.ResizeEdge
	}{
		{"top left", fyne.NewPos(2, 2), nativeglfw.ResizeEdgeTopLeft},
		{"top right", fyne.NewPos(98, 2), nativeglfw.ResizeEdgeTopRight},
		{"bottom left", fyne.NewPos(2, 78), nativeglfw.ResizeEdgeBottomLeft},
		{"bottom right", fyne.NewPos(98, 78), nativeglfw.ResizeEdgeBottomRight},
		{"left", fyne.NewPos(2, 40), nativeglfw.ResizeEdgeLeft},
		{"right", fyne.NewPos(98, 40), nativeglfw.ResizeEdgeRight},
		{"top", fyne.NewPos(50, 2), nativeglfw.ResizeEdgeTop},
		{"bottom", fyne.NewPos(50, 78), nativeglfw.ResizeEdgeBottom},
		{"border inclusive", fyne.NewPos(8, 40), nativeglfw.ResizeEdgeLeft},
		{"outside border", fyne.NewPos(9, 9), nativeglfw.ResizeEdgeNone},
		{"center", fyne.NewPos(50, 40), nativeglfw.ResizeEdgeNone},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, waylandResizeEdgeAt(test.pos, size))
		})
	}
}

func TestWaylandResizeCursorShape(t *testing.T) {
	tests := []struct {
		name string
		edge nativeglfw.ResizeEdge
		want nativeglfw.StandardCursor
		ok   bool
	}{
		{"left", nativeglfw.ResizeEdgeLeft, nativeglfw.ResizeEWCursor, true},
		{"right", nativeglfw.ResizeEdgeRight, nativeglfw.ResizeEWCursor, true},
		{"top", nativeglfw.ResizeEdgeTop, nativeglfw.ResizeNSCursor, true},
		{"bottom", nativeglfw.ResizeEdgeBottom, nativeglfw.ResizeNSCursor, true},
		{"top left", nativeglfw.ResizeEdgeTopLeft, nativeglfw.ResizeNWSECursor, true},
		{"bottom right", nativeglfw.ResizeEdgeBottomRight, nativeglfw.ResizeNWSECursor, true},
		{"top right", nativeglfw.ResizeEdgeTopRight, nativeglfw.ResizeNESWCursor, true},
		{"bottom left", nativeglfw.ResizeEdgeBottomLeft, nativeglfw.ResizeNESWCursor, true},
		{"none", nativeglfw.ResizeEdgeNone, 0, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := waylandResizeCursorShape(test.edge)
			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.want, got)
		})
	}
}
