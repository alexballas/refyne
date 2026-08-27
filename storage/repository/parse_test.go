package repository

import (
	"runtime"
	"testing"

	fyne "github.com/alexballas/refyne/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileURI(t *testing.T) {
	assert.Equal(t, "file:///tmp/foo.txt", NewFileURI("/tmp/foo.txt").String())
	assert.Equal(t, "file://C:/tmp/foo.txt", NewFileURI("C:/tmp/foo.txt").String())
}

func TestParseURI(t *testing.T) {
	uri, err := ParseURI("file:///tmp/foo.txt")
	assert.NoError(t, err)
	assert.Equal(t, "file:///tmp/foo.txt", uri.String())

	uri, err = ParseURI("file:/tmp/foo.txt")
	assert.NoError(t, err)
	assert.Equal(t, "file:///tmp/foo.txt", uri.String())

	uri, err = ParseURI("file://C:/tmp/foo.txt")
	assert.NoError(t, err)
	assert.Equal(t, "file://C:/tmp/foo.txt", uri.String())

	if runtime.GOOS == "windows" {
		uri, err = ParseURI(`file://C:\tmp\foo.txt`)
		assert.NoError(t, err)
		assert.Equal(t, "file://C:/tmp/foo.txt", uri.String())

		uri, err = ParseURI(`C:\tmp\foo.txt`)
		assert.NoError(t, err)
		assert.Equal(t, "file://C:/tmp/foo.txt", uri.String())
	}

	IPv6url := "http://[2001:db8:4006:812::200e]:8080/path/page.html"
	uri, err = ParseURI(IPv6url)
	assert.NoError(t, err)
	assert.Equal(t, IPv6url, uri.String())

	IPv6url = "http://[2001:db8:4006:812::200e]/path/page.html"
	uri, err = ParseURI(IPv6url)
	assert.NoError(t, err)
	assert.Equal(t, IPv6url, uri.String())
}

func TestParseInvalidURI(t *testing.T) {
	uri, err := ParseURI("/tmp/foo.txt")
	assert.Error(t, err)
	assert.Nil(t, uri)

	uri, err = ParseURI("file")
	assert.Error(t, err)
	assert.Nil(t, uri)

	uri, err = ParseURI("file:")
	assert.Error(t, err)
	assert.Nil(t, uri)

	uri, err = ParseURI("file://")
	assert.Error(t, err)
	assert.Nil(t, uri)

	uri, err = ParseURI(":foo")
	assert.Error(t, err)
	assert.Nil(t, uri)

	uri, err = ParseURI("scheme://0[]/invalid")
	assert.Error(t, err)
	assert.Nil(t, uri)
}

func TestParseURIMatrix(t *testing.T) {
	tests := []struct {
		name                    string
		input, serialized       string
		scheme, authority, path string
		query, fragment         string
	}{
		{
			name: "escaped file path", input: "file:///tmp/a%20%23%25%3F%E2%98%83.txt",
			serialized: "file:///tmp/a%20%23%25%3F%E2%98%83.txt", scheme: "file", path: "/tmp/a #%?☃.txt",
		},
		{
			name: "file query and fragment", input: "file:///tmp/media.mp4?token=a%20b#chapter-1",
			serialized: "file:///tmp/media.mp4?token=a%20b#chapter-1", scheme: "file", path: "/tmp/media.mp4",
			query: "token=a%20b", fragment: "chapter-1",
		},
		{
			name: "relative file path", input: "file://./testdata/media%20file.mp4?token=a%20b#chapter-1",
			serialized: "file://./testdata/media%20file.mp4?token=a%20b#chapter-1", scheme: "file", path: "./testdata/media file.mp4",
			query: "token=a%20b", fragment: "chapter-1",
		},
		{
			name: "parent relative file path", input: "file://../media.mp4",
			serialized: "file://../media.mp4", scheme: "file", path: "../media.mp4",
		},
		{
			name: "UNC path", input: "file://server/share/movie.mp4",
			serialized: "file://server/share/movie.mp4", scheme: "file", authority: "server", path: "/share/movie.mp4",
		},
		{
			name: "userinfo IPv4 port", input: "https://user:pass@127.0.0.1:8042/a%2Fb?q=a%20b#frag",
			serialized: "https://user:pass@127.0.0.1:8042/a%2Fb?q=a%20b#frag", scheme: "https",
			authority: "user:pass@127.0.0.1:8042", path: "/a/b", query: "q=a%20b", fragment: "frag",
		},
		{
			name: "IPv6", input: "http://[2001:db8::1]:8080/path",
			serialized: "http://[2001:db8::1]:8080/path", scheme: "http", authority: "[2001:db8::1]:8080", path: "/path",
		},
		{
			name: "URN", input: "urn:example:animal:ferret:nose?name=ferret#nose",
			serialized: "urn:example:animal:ferret:nose?name=ferret#nose", scheme: "urn",
			path: "example:animal:ferret:nose", query: "name=ferret", fragment: "nose",
		},
		{
			name: "opaque scheme", input: "mailto:user@example.com?subject=hello#top",
			serialized: "mailto:user@example.com?subject=hello#top", scheme: "mailto",
			path: "user@example.com", query: "subject=hello", fragment: "top",
		},
		{
			name: "Android content", input: "content://com.example.provider/media/42?mode=read#item",
			serialized: "content://com.example.provider/media/42?mode=read#item", scheme: "content",
			authority: "com.example.provider", path: "/media/42", query: "mode=read", fragment: "item",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u, err := ParseURI(test.input)
			require.NoError(t, err)
			assert.Equal(t, test.serialized, u.String())
			assert.Equal(t, test.scheme, u.Scheme())
			assert.Equal(t, test.authority, u.Authority())
			assert.Equal(t, test.path, u.Path())
			assert.Equal(t, test.query, u.Query())
			assert.Equal(t, test.fragment, u.Fragment())
		})
	}
}

func TestNewFileURIReservedCharactersAndUNC(t *testing.T) {
	u := NewFileURI("/tmp/a #%?☃.txt")
	assert.Equal(t, "file:///tmp/a%20%23%25%3F%E2%98%83.txt", u.String())
	assert.Equal(t, "/tmp/a #%?☃.txt", u.Path())
	assert.Equal(t, "a #%?☃.txt", u.Name())
	assert.Equal(t, ".txt", u.Extension())

	unc := NewFileURI("//server/share/movie.mp4")
	assert.Equal(t, "server", unc.Authority())
	assert.Equal(t, "/share/movie.mp4", unc.Path())
	assert.Equal(t, "file://server/share/movie.mp4", unc.String())
}

type recordingURIRepository struct {
	Repository
	input string
}

func (r *recordingURIRepository) ParseURI(input string) (fyne.URI, error) {
	r.input = input
	return NewFileURI("/custom"), nil
}

func TestParseURICustomRepositoryGetsOriginalString(t *testing.T) {
	const scheme = "original-string-test"
	repo := &recordingURIRepository{}
	Register(scheme, repo)
	t.Cleanup(func() { delete(repositoryTable, scheme) })

	input := scheme + "://host/a%2Fb?q=x%20y#frag"
	_, err := ParseURI(input)
	require.NoError(t, err)
	assert.Equal(t, input, repo.input)
}
