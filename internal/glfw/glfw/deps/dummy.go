//go:build required
// +build required

// Package dummy prevents go tooling from stripping the c dependencies.
package dummy

import (
	// Prevent go tooling from stripping out the c source files.
	_ "github.com/alexballas/refyne/v2/internal/glfw/glfw/deps/glad"
	_ "github.com/alexballas/refyne/v2/internal/glfw/glfw/deps/mingw"
	_ "github.com/alexballas/refyne/v2/internal/glfw/glfw/deps/wayland"
)
