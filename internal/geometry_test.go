package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fyne "github.com/alexballas/refyne/v2"
)

func TestMaxSizes(t *testing.T) {
	tests := []struct {
		name string
		a, b fyne.Size
	}{
		{name: "mixed", a: fyne.NewSize(1, 40), b: fyne.NewSize(30, 2)},
		{name: "zero", a: fyne.NewSize(0, 0), b: fyne.NewSize(4, 5)},
		{name: "negative", a: fyne.NewSize(-8, -2), b: fyne.NewSize(-3, -9)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.a.Max(test.b), MaxSizes(test.a, test.b))
		})
	}
}

func TestMinSizes(t *testing.T) {
	tests := []struct {
		name string
		a, b fyne.Size
	}{
		{name: "mixed", a: fyne.NewSize(1, 40), b: fyne.NewSize(30, 2)},
		{name: "zero", a: fyne.NewSize(0, 0), b: fyne.NewSize(4, 5)},
		{name: "negative", a: fyne.NewSize(-8, -2), b: fyne.NewSize(-3, -9)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.a.Min(test.b), MinSizes(test.a, test.b))
		})
	}
}
