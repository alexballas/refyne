package mobile

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/alexballas/refyne/v2/storage"
)

// recorder collects what the application handler saw. "Delivered" means the
// handler ran, not that the URI reached the mailbox.
type recorder struct {
	mu    sync.Mutex
	uris  []string
	mimes []string
}

func (r *recorder) handle(uri fyne.URI, mime string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.uris = append(r.uris, uri.String())
	r.mimes = append(r.mimes, mime)
}

func (r *recorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.uris...)
}

func testURI(t *testing.T, path string) fyne.URI {
	t.Helper()

	uri, err := storage.ParseURI(path)
	assert.Nil(t, err)
	return uri
}

// direct stands in for the driver function queue once Run has built it.
func direct(fn func()) { fn() }

func TestIntentMailbox_DeliveredWhenHandlerRegisters(t *testing.T) {
	m := &uriIntentMailbox{}
	r := &recorder{}

	m.setRun(direct)
	m.deliver(testURI(t, "file:///video.mp4"), "video/mp4")
	assert.Empty(t, r.seen(), "no handler yet, nothing may run")

	m.setHandler(r.handle)
	assert.Equal(t, []string{"file:///video.mp4"}, r.seen())
	assert.Equal(t, []string{"video/mp4"}, r.mimes)
}

// The cold-start case: the intent beats both the handler and the queue, and must
// wait for whichever of the two arrives last.
func TestIntentMailbox_DeliveredOnLaterOfHandlerAndRun(t *testing.T) {
	t.Run("handler last", func(t *testing.T) {
		m := &uriIntentMailbox{}
		r := &recorder{}

		m.deliver(testURI(t, "file:///a.mp4"), "")
		m.setRun(direct)
		assert.Empty(t, r.seen())

		m.setHandler(r.handle)
		assert.Equal(t, []string{"file:///a.mp4"}, r.seen())
	})

	t.Run("run last", func(t *testing.T) {
		m := &uriIntentMailbox{}
		r := &recorder{}

		m.deliver(testURI(t, "file:///a.mp4"), "")
		m.setHandler(r.handle)
		assert.Empty(t, r.seen())

		m.setRun(direct)
		assert.Equal(t, []string{"file:///a.mp4"}, r.seen())
	})
}

func TestIntentMailbox_DeliveredImmediatelyWhenReady(t *testing.T) {
	m := &uriIntentMailbox{}
	r := &recorder{}

	m.setRun(direct)
	m.setHandler(r.handle)

	m.deliver(testURI(t, "file:///a.mp4"), "video/mp4")
	assert.Equal(t, []string{"file:///a.mp4"}, r.seen())
}

func TestIntentMailbox_LatestWins(t *testing.T) {
	m := &uriIntentMailbox{}
	r := &recorder{}

	m.deliver(testURI(t, "file:///first.mp4"), "")
	m.deliver(testURI(t, "file:///second.mp4"), "")

	m.setRun(direct)
	m.setHandler(r.handle)
	assert.Equal(t, []string{"file:///second.mp4"}, r.seen(), "the superseded intent must never run")
}

// An intent arriving before Run must not be dropped, and must not be handled on
// the delivering thread - that is the promise SetOnOpenURI makes.
func TestIntentMailbox_NotRunInlineBeforeRun(t *testing.T) {
	m := &uriIntentMailbox{}
	r := &recorder{}

	m.setHandler(r.handle)
	m.deliver(testURI(t, "file:///a.mp4"), "")
	assert.Empty(t, r.seen())

	var queued []func()
	m.setRun(func(fn func()) { queued = append(queued, fn) })

	assert.Len(t, queued, 1, "delivery must go through the queue")
	assert.Empty(t, r.seen(), "handler must not have run before the queue was drained")

	queued[0]()
	assert.Equal(t, []string{"file:///a.mp4"}, r.seen())
}

// Application code that calls back into the mailbox - registering a new handler,
// say - would deadlock if the lock were still held.
func TestIntentMailbox_HandlerRunsWithoutLockHeld(t *testing.T) {
	m := &uriIntentMailbox{}

	locked := false
	m.setRun(direct)
	m.setHandler(func(fyne.URI, string) {
		locked = m.mu.TryLock()
		if locked {
			m.mu.Unlock()
		}
	})

	m.deliver(testURI(t, "file:///a.mp4"), "")
	assert.True(t, locked, "mailbox must be unlocked while the handler runs")
}

func TestIntentMailbox_ConcurrentDeliveryAndRegistration(t *testing.T) {
	m := &uriIntentMailbox{}
	r := &recorder{}
	uri := testURI(t, "file:///a.mp4")

	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); m.setRun(direct) }()
	go func() { defer wg.Done(); m.setHandler(r.handle) }()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			m.deliver(uri, "video/mp4")
		}
	}()
	wg.Wait()

	// Whatever the interleaving, the last intent must not be left sitting in the
	// mailbox once both gates are open.
	m.setRun(direct)
	m.setHandler(r.handle)
	assert.NotEmpty(t, r.seen())
}
