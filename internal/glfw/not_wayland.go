//go:build (linux && x11 && !wayland) || ((freebsd || netbsd || openbsd) && !wayland)

package glfw

const WAYLAND = false
