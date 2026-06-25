package fyne

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContainer_Accessibility(t *testing.T) {
	c := NewContainerWithoutLayout()

	// Container conforms to the Accessible interface.
	var a Accessible = c
	assert.Equal(t, "Container", a.AccessibilityLabel())
	assert.Equal(t, AccessibleRoleContainer, a.AccessibilityRole())
}
