package cache

import (
	"testing"
	"time"

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

func TestAttachCanvasRef(t *testing.T) {
	testClearAll()
	defer testClearAll()

	c1, c2 := &dummyCanvas{}, &dummyCanvas{}
	obj := &dummyWidget{}
	var ref CanvasRef

	assert.True(t, AttachCanvasRef(&ref, obj, c1))
	assert.Same(t, c1, GetCanvasForObject(obj))
	first := ref.info
	assert.False(t, AttachCanvasRef(&ref, obj, c1), "the registration kept is used again")
	assert.Same(t, first, ref.info)

	SetCanvasForObject(obj, c2, nil)
	assert.Same(t, c2, GetCanvasForObject(obj))
	assert.True(t, AttachCanvasRef(&ref, obj, c1), "moved to another canvas, so registered anew")
	assert.Same(t, c1, GetCanvasForObject(obj))
	assert.NotSame(t, first, ref.info)

	ref.info.expires.Store(0)
	destroyExpiredCanvases(time.Now())
	assert.Nil(t, GetCanvasForObject(obj))
	assert.True(t, AttachCanvasRef(&ref, obj, c1), "expired and cleaned out, so registered anew")
	assert.Same(t, c1, GetCanvasForObject(obj))

	CleanCanvas(c1)
	assert.Nil(t, GetCanvasForObject(obj))
	assert.True(t, AttachCanvasRef(&ref, obj, c1), "cleaned with its canvas, so registered anew")
	assert.Same(t, c1, GetCanvasForObject(obj))
}
