//go:build wasm || test_web_driver

package glfw

func (*window) processFramebufferResize(int, int) bool { return false }

func (*window) settleFramebufferResize(func()) bool { return false }
