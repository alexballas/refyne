package canvas

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAdvanceShaderTime(t *testing.T) {
	var elapsed time.Duration
	var lastTick time.Time
	base := time.Unix(100, 0)

	elapsed, lastTick = advanceShaderTime(elapsed, lastTick, base)
	assert.Equal(t, time.Duration(0), elapsed)

	elapsed, lastTick = advanceShaderTime(elapsed, lastTick, base.Add(16*time.Millisecond))
	assert.Equal(t, 16*time.Millisecond, elapsed)

	elapsed, lastTick = advanceShaderTime(elapsed, lastTick, base.Add(16*time.Millisecond+5*time.Second))
	assert.Equal(t, 16*time.Millisecond, elapsed)

	elapsed, _ = advanceShaderTime(elapsed, lastTick, base.Add(32*time.Millisecond+5*time.Second))
	assert.Equal(t, 32*time.Millisecond, elapsed)
}
