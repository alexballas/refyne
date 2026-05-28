package repository

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexballas/refyne/v2/storage"
	"github.com/alexballas/refyne/v2/storage/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileRepositoryReaderSeeker(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "media.bin")
	content := []byte("0123456789ABCDEF")
	require.NoError(t, os.WriteFile(p, content, 0o644))

	repository.Register("file", NewFileRepository())
	u := storage.NewFileURI(p)

	rs, err := storage.ReaderSeeker(u)
	require.NoError(t, err)
	defer rs.Close()

	// io.SeekEnd reports the size - this is how http.ServeContent sizes content.
	end, err := rs.Seek(0, io.SeekEnd)
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), end)

	// A read at EOF returns io.EOF.
	_, err = rs.Read(make([]byte, 1))
	assert.Equal(t, io.EOF, err)

	// io.SeekStart rewinds to the beginning (ServeContent's seek-back step).
	off, err := rs.Seek(0, io.SeekStart)
	require.NoError(t, err)
	assert.Equal(t, int64(0), off)
	head := make([]byte, 4)
	_, err = io.ReadFull(rs, head)
	require.NoError(t, err)
	assert.Equal(t, []byte("0123"), head)

	// io.SeekStart to an absolute offset.
	_, err = rs.Seek(10, io.SeekStart)
	require.NoError(t, err)
	mid := make([]byte, 3)
	_, err = io.ReadFull(rs, mid)
	require.NoError(t, err)
	assert.Equal(t, []byte("ABC"), mid)

	// io.SeekCurrent relative to the current offset (now 13).
	cur, err := rs.Seek(-2, io.SeekCurrent)
	require.NoError(t, err)
	assert.Equal(t, int64(11), cur)
	one := make([]byte, 1)
	_, err = io.ReadFull(rs, one)
	require.NoError(t, err)
	assert.Equal(t, []byte("B"), one)

	// The URI is preserved through the wrapper.
	assert.Equal(t, u.String(), rs.URI().String())
}

func TestFileRepositoryReaderSeekerOffsetIndependence(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	require.NoError(t, os.WriteFile(p, []byte("ABCDEFGH"), 0o644))
	repository.Register("file", NewFileRepository())
	u := storage.NewFileURI(p)

	a, err := storage.ReaderSeeker(u)
	require.NoError(t, err)
	defer a.Close()
	b, err := storage.ReaderSeeker(u)
	require.NoError(t, err)
	defer b.Close()

	_, err = a.Seek(4, io.SeekStart)
	require.NoError(t, err)

	ba := make([]byte, 1)
	bb := make([]byte, 1)
	_, err = io.ReadFull(a, ba)
	require.NoError(t, err)
	_, err = io.ReadFull(b, bb)
	require.NoError(t, err)
	assert.Equal(t, []byte("E"), ba)
	assert.Equal(t, []byte("A"), bb) // b is unaffected by a's seek
}

func TestFileRepositoryReaderSeekerClosed(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	require.NoError(t, os.WriteFile(p, []byte("data"), 0o644))
	repository.Register("file", NewFileRepository())

	rs, err := storage.ReaderSeeker(storage.NewFileURI(p))
	require.NoError(t, err)
	require.NoError(t, rs.Close())

	_, err = rs.Read(make([]byte, 1))
	assert.Error(t, err)
	_, err = rs.Seek(0, io.SeekStart)
	assert.Error(t, err)
}

func TestReaderSeekerServeContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "media.bin")
	content := []byte("0123456789ABCDEF")
	require.NoError(t, os.WriteFile(p, content, 0o644))
	repository.Register("file", NewFileRepository())
	u := storage.NewFileURI(p)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs, err := storage.ReaderSeeker(u)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rs.Close()
		http.ServeContent(w, r, u.Name(), time.Time{}, rs)
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Range", "bytes=4-7")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
	assert.Equal(t, "bytes 4-7/16", resp.Header.Get("Content-Range"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, []byte("4567"), body)
}

func TestReaderSeekerUnsupported(t *testing.T) {
	repository.Register("mem", NewInMemoryRepository("mem"))
	u, err := storage.ParseURI("mem:///foo")
	require.NoError(t, err)

	_, err = storage.ReaderSeeker(u)
	assert.True(t, errors.Is(err, repository.ErrOperationNotSupported))
}
