package cache

import "sync/atomic"

var (
	minSizeGen    atomic.Uint64
	minSizePasses atomic.Uint64
)

// BeginMinSizePass starts a pass in which a widget computes its MinSize once.
func BeginMinSizePass() {
	minSizeGen.Store(minSizePasses.Add(1))
}

// EndMinSizePass ends the current min size pass.
func EndMinSizePass() {
	minSizeGen.Store(0)
}

// MinSizeGeneration returns the current min size pass, or 0 outside a pass.
func MinSizeGeneration() uint64 {
	return minSizeGen.Load()
}

// RestartMinSizePass makes widgets compute their MinSize again in the current
// pass, as a widget resized or refreshed in it may change the min size of those
// above it that were already computed.
func RestartMinSizePass() {
	if gen := minSizeGen.Load(); gen != 0 {
		minSizeGen.CompareAndSwap(gen, minSizePasses.Add(1))
	}
}
