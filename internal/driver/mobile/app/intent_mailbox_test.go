package app

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntentMailbox_BuffersUntilCallbackRegisters(t *testing.T) {
	m := &intentMailbox{}

	var got []string
	m.deliver("content://media/1", "video/mp4")
	m.setCallback(func(uri, mime string) { got = append(got, uri+" "+mime) })

	assert.Equal(t, []string{"content://media/1 video/mp4"}, got)
}

func TestIntentMailbox_DeliversImmediatelyOnceRegistered(t *testing.T) {
	m := &intentMailbox{}

	var got []string
	m.setCallback(func(uri, mime string) { got = append(got, uri) })
	m.deliver("content://media/1", "")

	assert.Equal(t, []string{"content://media/1"}, got)
}

func TestIntentMailbox_LatestWins(t *testing.T) {
	m := &intentMailbox{}

	var got []string
	m.deliver("content://media/1", "")
	m.deliver("content://media/2", "")
	m.setCallback(func(uri, mime string) { got = append(got, uri) })

	assert.Equal(t, []string{"content://media/2"}, got)
}

func TestIntentMailbox_CallbackRunsWithoutLockHeld(t *testing.T) {
	m := &intentMailbox{}

	locked := false
	m.setCallback(func(uri, mime string) {
		locked = m.mu.TryLock()
		if locked {
			m.mu.Unlock()
		}
	})
	m.deliver("content://media/1", "")

	assert.True(t, locked, "mailbox must be unlocked while the callback runs")
}

func TestIntentMailbox_ConcurrentDeliveryAndRegistration(t *testing.T) {
	m := &intentMailbox{}

	var mu sync.Mutex
	var count int

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		m.setCallback(func(uri, mime string) {
			mu.Lock()
			count++
			mu.Unlock()
		})
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			m.deliver("content://media/1", "video/mp4")
		}
	}()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.NotZero(t, count)
}
