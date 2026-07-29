package software

import (
	"image"
	"testing"
)

func TestApplyRoundedCornersClippedRect(t *testing.T) {
	// A 40x40 rectangle of which only the left half survived clipping, so the
	// edge at x=19 is a clip boundary rather than a corner of the rectangle.
	rect := image.Rect(0, 0, 40, 40)
	img := image.NewNRGBA(image.Rect(0, 0, 20, 40))
	for i := range img.Pix {
		img.Pix[i] = 255
	}

	applyRoundedCorners(img, rect, 10)

	alphaAt := func(x, y int) uint8 {
		return img.Pix[img.PixOffset(x, y)+3]
	}

	for _, corner := range []image.Point{{X: 0, Y: 0}, {X: 0, Y: 39}} {
		if got := alphaAt(corner.X, corner.Y); got != 0 {
			t.Fatalf("corner %v of the rectangle should be rounded away, got alpha %d", corner, got)
		}
	}

	for _, edge := range []image.Point{{X: 19, Y: 0}, {X: 19, Y: 39}} {
		if got := alphaAt(edge.X, edge.Y); got != 255 {
			t.Fatalf("clipped edge %v must stay square, got alpha %d", edge, got)
		}
	}
}
