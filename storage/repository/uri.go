package repository

import (
	"bufio"
	"mime"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"

	fyne "github.com/alexballas/refyne/v2"
)

// EqualURI returns true if the two URIs are equal.
//
// Since: 2.6
func EqualURI(t1, t2 fyne.URI) bool {
	if t1 == nil || t2 == nil {
		return t1 == t2
	}

	u1, ok1 := t1.(*uri)
	u2, ok2 := t2.(*uri)
	if ok1 && ok2 {
		if u1 == u2 {
			return true
		}

		first, second := u1.URL, u2.URL
		if !equalUserinfo(first.User, second.User) {
			return false
		}
		first.User, second.User = nil, nil
		return first == second
	}

	return t1 == t2 || t1.String() == t2.String()
}

func equalUserinfo(first, second *url.Userinfo) bool {
	if first == nil || second == nil {
		return first == second
	}
	return first.String() == second.String()
}

var _ fyne.URI = &uri{}

type uri struct {
	url.URL
}

func (u *uri) Extension() string {
	return path.Ext(u.Path())
}

func (u *uri) Name() string {
	return path.Base(u.Path())
}

func (u *uri) MimeType() string {
	mimeTypeFull := mime.TypeByExtension(u.Extension())
	if mimeTypeFull == "" {
		mimeTypeFull = "text/plain"

		repo, err := ForURI(u)
		if err != nil {
			return "application/octet-stream"
		}

		readCloser, err := repo.Reader(u)
		if err == nil {
			defer readCloser.Close()
			scanner := bufio.NewScanner(readCloser)
			if scanner.Scan() && !utf8.Valid(scanner.Bytes()) {
				mimeTypeFull = "application/octet-stream"
			}
		}
	}

	mimeType, _, _ := strings.Cut(mimeTypeFull, ";")
	return mimeType
}

func (u *uri) Scheme() string {
	return u.URL.Scheme
}

func (u *uri) Authority() string {
	if u.User != nil {
		if user := u.User.String(); user != "" {
			return user + "@" + u.Host
		}
	}
	return u.Host
}

func (u *uri) Path() string {
	if u.URL.Opaque != "" {
		return u.URL.Opaque
	}
	return u.URL.Path
}

func (u *uri) Query() string {
	return u.RawQuery
}

func (u *uri) Fragment() string {
	return u.URL.Fragment
}
