//go:build !((linux && (wayland || !x11)) || (freebsd && wayland) || (netbsd && wayland) || (openbsd && wayland))

package glfw

const WAYLAND = false
