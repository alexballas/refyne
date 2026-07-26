package mobile

import (
	"sync"

	fyne "github.com/alexballas/refyne/v2"
)

type pendingURI struct {
	uri  fyne.URI
	mime string
}

// uriIntentMailbox carries a URI handed to us by another app - an Android share
// or open-with - through to the application callback.
//
// Two things must exist before that callback can run, and neither is in place
// when a cold-start intent arrives: the application has to have registered a
// handler, which it can only do once its UI exists, and the driver has to have
// built its function queue in Run, without which a hand-off would run inline on
// the calling thread instead of the Fyne goroutine. So the mailbox holds the
// intent until both are present, and both edges try to flush. Only the most
// recent intent is kept: one nobody has taken yet is already superseded.
type uriIntentMailbox struct {
	mu      sync.Mutex
	handler func(fyne.URI, string)
	run     func(func()) // hands work to the Fyne goroutine; nil until Run
	pending *pendingURI
}

// setHandler registers the application callback, delivering an intent that
// arrived before it existed.
func (m *uriIntentMailbox) setHandler(fn func(fyne.URI, string)) {
	m.mu.Lock()
	m.handler = fn
	flush := m.takeLocked()
	m.mu.Unlock()

	flush()
}

// setRun records how to reach the Fyne goroutine. Called from Run once the
// function queue exists.
func (m *uriIntentMailbox) setRun(run func(func())) {
	m.mu.Lock()
	m.run = run
	flush := m.takeLocked()
	m.mu.Unlock()

	flush()
}

// deliver accepts an intent from the OS. It is called on an arbitrary thread.
func (m *uriIntentMailbox) deliver(uri fyne.URI, mime string) {
	m.mu.Lock()
	m.pending = &pendingURI{uri: uri, mime: mime}
	flush := m.takeLocked()
	m.mu.Unlock()

	flush()
}

// takeLocked consumes the pending intent when it can be delivered, returning the
// work to perform once the caller has released the lock. Application code must
// never run while the mailbox is locked. The returned function is never nil.
func (m *uriIntentMailbox) takeLocked() func() {
	if m.handler == nil || m.run == nil || m.pending == nil {
		return func() {}
	}

	handler, pending, run := m.handler, *m.pending, m.run
	m.pending = nil

	return func() {
		run(func() {
			handler(pending.uri, pending.mime)
		})
	}
}
