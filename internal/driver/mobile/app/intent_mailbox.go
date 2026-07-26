package app

import "sync"

// sharedIntent is a URI handed to this app by another one, together with the
// MIME type the sender declared. The MIME may be empty.
type sharedIntent struct {
	uri  string
	mime string
}

// intentMailbox holds the most recent incoming intent until a consumer exists to
// take it.
//
// Unlike the file picker, which sets a callback and then asks for a file, intents
// are pushed at us: a cold-start share reaches the Java layer before the driver
// has finished starting, so dropping deliveries that arrive with no callback set
// would lose exactly the case this exists for. Only the latest intent is kept -
// an older one that nobody has taken yet has already been superseded.
type intentMailbox struct {
	mu       sync.Mutex
	callback func(uri, mime string)
	pending  *sharedIntent
}

var theIntentMailbox intentMailbox

// SetIntentCallback registers fn to receive share and open-with intents from the
// OS. If one arrived before registration it is delivered immediately, on the
// calling goroutine. Passing nil stops delivery.
func SetIntentCallback(fn func(uri, mime string)) {
	theIntentMailbox.setCallback(fn)
}

func (m *intentMailbox) setCallback(fn func(uri, mime string)) {
	m.mu.Lock()
	m.callback = fn
	deliver, pending := m.takeLocked()
	m.mu.Unlock()

	if deliver != nil {
		deliver(pending.uri, pending.mime)
	}
}

func (m *intentMailbox) deliver(uri, mime string) {
	m.mu.Lock()
	m.pending = &sharedIntent{uri: uri, mime: mime}
	deliver, pending := m.takeLocked()
	m.mu.Unlock()

	if deliver != nil {
		deliver(pending.uri, pending.mime)
	}
}

// takeLocked consumes the pending intent if it can be delivered now, returning
// the callback to invoke once the caller has released the lock. Application code
// must never run while the mailbox is locked.
func (m *intentMailbox) takeLocked() (func(uri, mime string), sharedIntent) {
	if m.callback == nil || m.pending == nil {
		return nil, sharedIntent{}
	}

	pending := *m.pending
	m.pending = nil
	return m.callback, pending
}
