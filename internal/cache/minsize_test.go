package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinSizePass(t *testing.T) {
	assert.Zero(t, MinSizeGeneration())
	RestartMinSizePass()
	assert.Zero(t, MinSizeGeneration(), "a restart outside a pass does not start one")

	BeginMinSizePass()
	first := MinSizeGeneration()
	assert.NotZero(t, first)
	RestartMinSizePass()
	restarted := MinSizeGeneration()
	assert.NotZero(t, restarted)
	assert.NotEqual(t, first, restarted)
	EndMinSizePass()
	assert.Zero(t, MinSizeGeneration())

	BeginMinSizePass()
	defer EndMinSizePass()
	assert.NotContains(t, []uint64{0, first, restarted}, MinSizeGeneration(), "no pass reuses a generation")
}
