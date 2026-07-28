package app

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestForegroundServiceLifecycle(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac is not installed")
	}
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java is not installed")
	}

	classes := t.TempDir()
	source := "ForegroundServiceLifecycle.java"
	harness := filepath.Join("testdata", "ForegroundServiceLifecycleTest.java")
	if output, err := exec.Command(
		javac, "-source", "1.8", "-target", "1.8", "-d", classes, source, harness,
	).CombinedOutput(); err != nil {
		t.Fatalf("compiling lifecycle test: %v\n%s", err, output)
	}
	if output, err := exec.Command(
		java, "-ea", "-cp", classes, "org.golang.app.ForegroundServiceLifecycleTest",
	).CombinedOutput(); err != nil {
		t.Fatalf("running lifecycle test: %v\n%s", err, output)
	}
}
