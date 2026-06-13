package animation

import (
	"testing"
	"time"

	fyne "github.com/alexballas/refyne/v2"
)

func TestRunnerRunning(t *testing.T) {
	r := Runner{}
	if r.Running() {
		t.Fatal("new runner reports running")
	}

	a := fyne.NewAnimation(time.Second, func(float32) {})
	r.Start(a)
	if !r.Running() {
		t.Fatal("runner with started animation reports not running")
	}

	r.Stop(a)
	if r.Running() {
		t.Fatal("runner with stopped animation reports running")
	}
}

func BenchmarkRunnerAllocs(b *testing.B) {
	r := Runner{}
	var fl float32
	// setup some animations
	for i := 0; i < 10; i++ {
		r.pendingAnimations = append(r.pendingAnimations, newAnim(
			fyne.NewAnimation(1000*time.Second, func(f float32) {
				fl = f
			}),
		))
	}
	for n := 0; n < b.N; n++ {
		r.runOneFrame()
	}

	b.ReportAllocs()
	fl = fl + 1 // dummy use of variable
}
