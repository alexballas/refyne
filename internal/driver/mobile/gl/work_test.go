//go:build cgo && (darwin || linux || openbsd || freebsd)

package gl

import (
	"sync"
	"testing"
	"time"
)

// newTestContext uses the production queue setup without issuing any GL calls.
func newTestContext() *context {
	_, worker := NewContext()
	return worker.(*context)
}

// TestWakeupsCoalescePerBurst covers N calls producing one wakeup, not N.
func TestWakeupsCoalescePerBurst(t *testing.T) {
	ctx := newTestContext()

	// Below the queue capacity, so the test stays synchronous. Backpressure is
	// covered by TestConcurrentProducersNeverStrandWork.
	const frameCalls = 200
	for i := 0; i < frameCalls; i++ {
		ctx.enqueue(call{})
	}

	if len(ctx.work) == 0 {
		t.Fatal("no work queued after enqueueing work")
	}
	if got := len(ctx.workAvailable); got != 1 {
		t.Errorf("pending wakeups after %d enqueues = %d, want 1", frameCalls, got)
	}
	if got := len(ctx.work); got != frameCalls {
		t.Errorf("queued calls = %d, want %d", got, frameCalls)
	}
}

// TestSignalIsIdempotentWhileWorkerAwake covers that calls made while the worker
// is awake queue no further wakeups.
func TestSignalIsIdempotentWhileWorkerAwake(t *testing.T) {
	ctx := newTestContext()

	ctx.enqueue(call{})
	ctx.enqueue(call{})
	<-ctx.workAvailable // the worker is now awake and busy

	for i := 0; i < 50; i++ {
		ctx.enqueue(call{})
	}
	if got := len(ctx.workAvailable); got != 0 {
		t.Errorf("pending wakeups while worker is awake = %d, want 0", got)
	}

	// Publishing idle then re-checking must find the queued calls.
	if ctx.drained() {
		t.Error("drained() = true although calls are queued, work would be stranded")
	}
}

// TestNoLostWakeupOnGoingIdle covers the ordering in drained: a call enqueued
// after the worker's last receive is not signalled, so the worker has to find it.
func TestNoLostWakeupOnGoingIdle(t *testing.T) {
	ctx := newTestContext()

	ctx.enqueue(call{})
	<-ctx.workAvailable
	<-ctx.work // the worker consumed its only call

	if ctx.consumerIdle.Load() {
		t.Fatal("test setup: worker should be marked awake after being woken")
	}

	// This producer sees consumerIdle == false, so it must not signal.
	ctx.enqueue(call{})
	if got := len(ctx.workAvailable); got != 0 {
		t.Fatalf("producer signalled while worker awake, wakeups = %d, want 0", got)
	}

	if ctx.drained() {
		t.Fatal("drained() = true with a call enqueued while going idle, work is lost")
	}
	if len(ctx.work) == 0 {
		t.Error("queued call disappeared")
	}
}

// TestIdlePublishesStateForNextEnqueue covers that a parked worker is woken again.
func TestIdlePublishesStateForNextEnqueue(t *testing.T) {
	ctx := newTestContext()

	ctx.enqueue(call{})
	<-ctx.workAvailable
	<-ctx.work

	ctx.DoWork() // an empty queue must return and re-arm the worker
	if !ctx.consumerIdle.Load() {
		t.Error("consumerIdle = false after going idle, producers would not wake the worker")
	}

	ctx.enqueue(call{})
	if got := len(ctx.workAvailable); got != 1 {
		t.Errorf("pending wakeups after enqueue to a parked worker = %d, want 1", got)
	}
}

// Blocking calls must wait for their result even when notifications coalesce.
func TestBlockingCallsWaitForResult(t *testing.T) {
	ctx := newTestContext()
	result := make(chan uintptr, 1)
	go func() {
		result <- ctx.enqueue(call{blocking: true})
	}()

	select {
	case <-ctx.WorkAvailable():
	case <-time.After(5 * time.Second):
		t.Fatal("blocking call did not wake the worker")
	}
	if c := <-ctx.work; !c.blocking {
		t.Fatal("queued call lost its blocking flag")
	}
	select {
	case <-result:
		t.Fatal("blocking call returned before its result was delivered")
	default:
	}

	select {
	case ctx.retvalue <- 17:
	case <-time.After(5 * time.Second):
		t.Fatal("blocking call did not receive its result")
	}
	select {
	case got := <-result:
		if got != 17 {
			t.Errorf("blocking call returned %d, want 17", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("blocking call did not return after receiving its result")
	}
}

// TestConcurrentProducersNeverStrandWork drives several producers while a worker
// drains. The total exceeds the queue capacity, so the producers block and are
// woken by the drain. The worker only ever waits on workAvailable, so the
// watchdog turns a lost wakeup into a failure instead of a hang.
func TestConcurrentProducersNeverStrandWork(t *testing.T) {
	ctx := newTestContext()

	const (
		producers      = 4
		callsPerWriter = 500
		total          = producers * callsPerWriter
	)

	var (
		mu   sync.Mutex
		seen int
	)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		// Mirrors DoWork: drain, then decide whether to go idle.
		for {
			for {
				select {
				case <-ctx.work:
					mu.Lock()
					seen++
					mu.Unlock()
					continue
				default:
				}
				break
			}

			if ctx.drained() {
				mu.Lock()
				finished := seen >= total
				mu.Unlock()
				if finished {
					return
				}
				select {
				case <-ctx.workAvailable:
				case <-time.After(5 * time.Second):
					if len(ctx.work) > 0 {
						t.Errorf("worker was not woken although %d calls are queued", len(ctx.work))
						return
					}
				}
			}
		}
	}()

	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < callsPerWriter; i++ {
				ctx.enqueue(call{})
			}
		}()
	}
	producersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(producersDone)
	}()
	select {
	case <-producersDone:
	case <-time.After(10 * time.Second):
		t.Fatal("producers blocked because the worker stopped draining")
	}

	select {
	case <-workerDone:
	case <-time.After(30 * time.Second):
		t.Fatal("worker did not finish, work was stranded")
	}

	mu.Lock()
	defer mu.Unlock()
	if seen != total {
		t.Errorf("worker processed %d calls, want %d", seen, total)
	}
}
