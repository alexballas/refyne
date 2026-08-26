package internal

import fyne "github.com/alexballas/refyne/v2"

// MaxSizes returns the element-wise maximum of the two sizes without boxing
// either value through the Vector2 interface used by Size.Max.
func MaxSizes(a, b fyne.Size) fyne.Size {
	return fyne.Size{Width: fyne.Max(a.Width, b.Width), Height: fyne.Max(a.Height, b.Height)}
}

// MinSizes returns the element-wise minimum of the two sizes without boxing
// either value through the Vector2 interface used by Size.Min.
func MinSizes(a, b fyne.Size) fyne.Size {
	return fyne.Size{Width: fyne.Min(a.Width, b.Width), Height: fyne.Min(a.Height, b.Height)}
}
