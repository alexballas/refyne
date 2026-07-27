// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mobile

import (
	"flag"

	"github.com/alexballas/refyne/v2/cmd/fyne/internal/metadata"
)

type command struct {
	run  func(*command) error
	Flag flag.FlagSet
	Name string

	IconPath, AppName      string
	Version, Cert, Profile string
	Build                  int

	iconFG, iconBG, iconMono string

	// androidMeta carries the [Android] section of FyneApp.toml, which gates the
	// optional manifest entries: share / open-with intent filters, the
	// foreground service and the permissions each of them needs. Nil for apps
	// that declare no [Android] section, and ignored on other platforms.
	androidMeta *metadata.Android
}
