//go:build wasm || test_web_driver

package glfw

import (
	"time"

	"github.com/alexballas/refyne/v2"
)

const webDefaultDoubleTapDelay = 300 * time.Millisecond

func (d *gLDriver) SetSystemTrayMenu(m *fyne.Menu) {
	// no-op for wasm apps using this driver
}

func (d *gLDriver) catchTerm() {}

func setDisableScreenBlank(disable bool) {
	// awaiting complete support for WakeLock
}

func (d *gLDriver) DoubleTapDelay() time.Duration {
	return webDefaultDoubleTapDelay
}
