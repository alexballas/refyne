package glfw

import fyne "github.com/alexballas/refyne/v2"

func (d *gLDriver) StartAnimation(a *fyne.Animation) {
	d.animation.Start(a)
	wakeEventLoop()
}

func (d *gLDriver) StopAnimation(a *fyne.Animation) {
	d.animation.Stop(a)
	wakeEventLoop()
}
