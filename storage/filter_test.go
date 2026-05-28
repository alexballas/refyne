package storage_test

import (
	"testing"

	"github.com/alexballas/refyne/v2/storage"

	_ "github.com/alexballas/refyne/v2/test"

	"github.com/stretchr/testify/assert"
)

func TestFileFilter(t *testing.T) {
	filter := storage.NewExtensionFileFilter([]string{".jpg", ".png"})

	assert.NotNil(t, filter)
	assert.True(t, filter.Matches(storage.NewURI("content:///otherapp/something/pic.JPG")))
	assert.True(t, filter.Matches(storage.NewURI("content:///otherapp/something/pic.jpg")))

	assert.True(t, filter.Matches(storage.NewURI("content:///otherapp/something/pic.PNG")))
	assert.True(t, filter.Matches(storage.NewURI("content:///otherapp/something/pic.png")))

	assert.False(t, filter.Matches(storage.NewURI("content:///otherapp/something/pic.TIFF")))
	assert.False(t, filter.Matches(storage.NewURI("content:///otherapp/something/pic.tiff")))
}
