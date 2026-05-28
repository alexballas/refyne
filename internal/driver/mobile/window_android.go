//go:build android

package mobile

import (
	fyneDriver "github.com/alexballas/refyne/v2/driver"
	"github.com/alexballas/refyne/v2/internal/driver/mobile/app"
)

// Assert we are satisfying the driver.NativeWindow interface
var _ fyneDriver.NativeWindow = (*window)(nil)

func (w *window) RunNative(f func(context any)) {
	app.RunOnJVM(func(vm, env, ctx uintptr) error {
		data := &fyneDriver.AndroidWindowContext{
			NativeWindow: w.handle,
			AndroidContext: fyneDriver.AndroidContext{
				VM:  vm,
				Env: env,
				Ctx: ctx,
			},
		}
		f(data)
		return nil
	})
}
