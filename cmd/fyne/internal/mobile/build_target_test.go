package mobile

import "testing"

func TestAndroidTargetSDK(t *testing.T) {
	tests := []struct {
		name         string
		release      bool
		distribution bool
		want         int
	}{
		{name: "debug APK", want: 29},
		{name: "release APK", release: true, want: 35},
		{name: "distribution bundle", distribution: true, want: 35},
		{name: "release distribution", release: true, distribution: true, want: 35},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := androidTargetSDK(test.release, test.distribution); got != test.want {
				t.Fatalf("androidTargetSDK() = %d, want %d", got, test.want)
			}
		})
	}
}
