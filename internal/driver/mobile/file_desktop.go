//go:build !ios && !android

package mobile

import (
	"io"

	"github.com/alexballas/refyne/v2"
	intRepo "github.com/alexballas/refyne/v2/internal/repository"
	"github.com/alexballas/refyne/v2/storage/repository"
)

func deleteURI(_ fyne.URI) error {
	// no-op as we use the internal FileRepository
	return nil
}

func existsURI(fyne.URI) (bool, error) {
	// no-op as we use the internal FileRepository
	return false, nil
}

func nativeFileOpen(*fileOpen) (io.ReadCloser, error) {
	// no-op as we use the internal FileRepository
	return nil, nil
}

func nativeFileSave(*fileSave, bool) (io.WriteCloser, error) {
	// no-op as we use the internal FileRepository
	return nil, nil
}

func registerRepository(_ *driver) {
	repository.Register("file", intRepo.NewFileRepository())
}
