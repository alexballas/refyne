package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachCanvasLifecycle(t *testing.T) {
	testClearAll()
	defer testClearAll()

	oldCanvas := &dummyCanvas{}
	newCanvas := &dummyCanvas{}
	obj := &dummyWidget{}

	called := 0
	setup := func() {
		called++
	}

	require.True(t, AttachCanvas(obj, oldCanvas), "first attach needs setup")
	setup()
	assert.Same(t, oldCanvas, GetCanvasForObject(obj))
	require.False(t, AttachCanvas(obj, oldCanvas), "same-canvas revisit must not need setup")
	assert.Equal(t, 1, called)

	SetCanvasForObject(obj, oldCanvas, setup)
	assert.Equal(t, 1, called)

	SetCanvasForObject(obj, newCanvas, setup)
	assert.Equal(t, 2, called)
	assert.Same(t, newCanvas, GetCanvasForObject(obj), "a real move must replace the cached canvas")

	CleanCanvas(oldCanvas)
	assert.Same(t, newCanvas, GetCanvasForObject(obj), "cleaning the old canvas must not detach the moved object")
	CleanCanvas(newCanvas)
	assert.Nil(t, GetCanvasForObject(obj))
}

func TestAttachCanvasRefreshesExpiry(t *testing.T) {
	testClearAll()
	defer testClearAll()

	tm := &timeMock{}
	tm.setTime(10, 0)
	obj := &dummyWidget{}
	canvas := &dummyCanvas{}
	require.True(t, AttachCanvas(obj, canvas))

	tm.advanceWallClock(ValidDuration / 2)
	BeginFrame()
	require.False(t, AttachCanvas(obj, canvas))

	tm.advanceWallClock(3 * ValidDuration / 4)
	destroyExpiredCanvases(tm.now)
	assert.Same(t, canvas, GetCanvasForObject(obj), "same-canvas use should extend expiry")

	tm.advanceWallClock(2 * ValidDuration)
	destroyExpiredCanvases(tm.now)
	assert.Nil(t, GetCanvasForObject(obj))
}
