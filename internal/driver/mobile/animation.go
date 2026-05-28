package mobile

import "github.com/alexballas/refyne/v2"

func (d *driver) StartAnimation(a *fyne.Animation) {
	d.animation.Start(a)
}

func (d *driver) StopAnimation(a *fyne.Animation) {
	d.animation.Stop(a)
}
