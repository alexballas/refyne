package async

import (
	"log"
	"runtime"
	"strings"
	"sync/atomic"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/internal/build"
)

// mainGoroutineID stores the main goroutine ID.
// This ID must be initialized during setup by calling `SetMainGoroutine` because
// a main goroutine may not equal to 1 due to the influence of a garbage collector.
var mainGoroutineID atomic.Uint64

func SetMainGoroutine() {
	mainGoroutineID.Store(goroutineID())
}

// isKnownMainGoroutine reports whether the caller is the main goroutine and the
// main goroutine has actually been identified yet.
//
// This is deliberately not IsMainGoroutine: the mobile build of that answers
// true for every caller until SetMainGoroutine runs, which suits EnsureMain
// (with no driver running there is nothing to hand work to, so running inline
// is right) but is wrong for EnsureNotMain. The lifecycle event pump is started
// by app.Run before the driver reaches SetMainGoroutine, and it dispatches with
// wait set, so during that window every queued event was reported as an illegal
// main-goroutine call and needlessly bounced onto a new goroutine.
func isKnownMainGoroutine() bool {
	id := mainGoroutineID.Load()
	return id != 0 && goroutineID() == id
}

// EnsureNotMain is part of our thread transition and makes sure that the passed function runs off main.
// If the context is running on a goroutine or the transition has been disabled this will blindly run.
// Otherwise, an error will be logged and the function will be called on a new goroutine.
//
// This will be removed later and should never be public
func EnsureNotMain(fn func()) {
	if build.MigratedToFyneDo() || !isKnownMainGoroutine() {
		fn()
		return
	}

	log.Println("*** Error in Fyne call thread, fyne.Do[AndWait] called from main goroutine ***")

	logStackTop(2)
	go fn()
}

// EnsureMain is part of our thread transition and makes sure that the passed function runs on main.
// If the context is main or the transition has been disabled this will blindly run.
// Otherwise, an error will be logged and the function will be called on the main goroutine.
//
// This will be removed later and should never be public
func EnsureMain(fn func()) {
	if build.MigratedToFyneDo() || IsMainGoroutine() {
		fn()
		return
	}

	log.Println("*** Error in Fyne call thread, this should have been called in fyne.Do[AndWait] ***")

	logStackTop(1)
	fyne.DoAndWait(fn)
}

// libraryPathPrefix is the source root of this module, derived from this file's
// own location rather than a hard coded path fragment. A literal would have to
// match the module path, so it silently stops matching in a fork, a vendored
// tree or a replace directive, and every threading error then gets reported
// against library internals instead of the caller that caused it.
var libraryPathPrefix = deriveLibraryPathPrefix()

func deriveLibraryPathPrefix() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}

	const self = "internal/async/"
	if i := strings.LastIndex(file, self); i != -1 {
		return file[:i]
	}

	return ""
}

// isLibraryFrame reports whether a source file belongs to this module. An
// undetectable prefix reports everything as caller code, so the immediate
// caller is named rather than something further up an unverified stack.
func isLibraryFrame(file string) bool {
	return libraryPathPrefix != "" && strings.HasPrefix(file, libraryPathPrefix)
}

func logStackTop(skip int) {
	pc := make([]uintptr, 16)
	n := runtime.Callers(skip, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, more := frames.Next()

	var nextFrame runtime.Frame
	for more {
		nextFrame, more = frames.Next()
		if nextFrame.File == "" || strings.Contains(nextFrame.File, "runtime") { // don't descend into Go
			break
		}

		frame = nextFrame
		if !isLibraryFrame(nextFrame.File) { // skip library lines
			break
		}
	}
	log.Printf("  From: %s:%d", frame.File, frame.Line)
}

func goroutineID() (id uint64) {
	var buf [30]byte
	runtime.Stack(buf[:], false)
	for i := 10; buf[i] != ' '; i++ {
		id = id*10 + uint64(buf[i]&15)
	}

	return id
}
