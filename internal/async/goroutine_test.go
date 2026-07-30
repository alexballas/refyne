package async

import (
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

var mainRoutineID uint64

func init() {
	mainRoutineID = goroutineID()
}

func TestGoroutineID(t *testing.T) {
	assert.Equal(t, uint64(1), mainRoutineID)

	var childID1, childID2 uint64
	testID1 := goroutineID()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		childID1 = goroutineID()
		wg.Done()
	}()
	go func() {
		childID2 = goroutineID()
		wg.Done()
	}()
	wg.Wait()
	testID2 := goroutineID()

	assert.Equal(t, testID1, testID2)
	assert.Greater(t, childID1, uint64(0))
	assert.NotEqual(t, testID1, childID1)
	assert.Greater(t, childID2, uint64(0))
	assert.NotEqual(t, childID1, childID2)
}

// app.Run starts the lifecycle event pump before the driver identifies the main
// goroutine, and that pump dispatches with wait set. Until SetMainGoroutine has
// run there is no main goroutine to be on, so EnsureNotMain must let those calls
// through untouched instead of reporting them as illegal and re-dispatching them.
func TestIsKnownMainGoroutine(t *testing.T) {
	orig := mainGoroutineID.Load()
	t.Cleanup(func() { mainGoroutineID.Store(orig) })

	mainGoroutineID.Store(0)
	assert.False(t, isKnownMainGoroutine(), "no goroutine is the main one before SetMainGoroutine")

	SetMainGoroutine()
	assert.True(t, isKnownMainGoroutine())

	other := make(chan bool)
	go func() { other <- isKnownMainGoroutine() }()
	assert.False(t, <-other, "a goroutine other than the recorded main one is never main")
}

// The frame reported for a threading error is only useful if library frames are
// skipped, which needs the module's real source root. A hard coded fragment
// stops matching whenever the module path does (fork, vendor, replace) and then
// blames library internals for every violation.
func TestIsLibraryFrameTracksThisModule(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed, cannot locate this test's source")
	}

	assert.NotEmpty(t, libraryPathPrefix, "the module source root must be detectable")
	assert.True(t, isLibraryFrame(file), "this module's own source must count as a library frame")
	assert.False(t, isLibraryFrame("/home/someone/app/main.go"), "application code must not")
}
