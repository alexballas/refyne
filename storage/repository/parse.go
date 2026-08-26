package repository

import (
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	fyne "github.com/alexballas/refyne/v2"
)

const domainLabelPattern = "[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?"

var rxHostName = regexp.MustCompile("^" + domainLabelPattern + `(?:\.` + domainLabelPattern + ")*$")

// NewFileURI implements the back-end logic to storage.NewFileURI, which you
// should use instead. This is only here because other functions in repository
// need to call it, and it prevents a circular import.
//
// Since: 2.0
func NewFileURI(path string) fyne.URI {
	if runtime.GOOS == "windows" {
		path = filepath.ToSlash(path)
	}
	if strings.HasPrefix(path, "//") {
		hostAndPath := strings.TrimPrefix(path, "//")
		host, uriPath, _ := strings.Cut(hostAndPath, "/")
		if host != "" && rxHostName.MatchString(host) {
			return &uri{URL: url.URL{Scheme: "file", Host: host, Path: "/" + uriPath}}
		}
	}

	return &uri{URL: url.URL{
		Scheme: "file",
		Path:   path,
	}}
}

// ParseURI implements the back-end logic for storage.ParseURI, which you
// should use instead. This is only here because other functions in repository
// need to call it, and it prevents a circular import.
//
// Since: 2.0
func ParseURI(s string) (fyne.URI, error) {
	scheme, uriPath, ok := strings.Cut(s, ":")
	if !ok || scheme == "" {
		return nil, errors.New("invalid URI, scheme must be present")
	}

	if strings.EqualFold(scheme, "urn") {
		fragmentParts := append(strings.SplitN(uriPath, "#", 2), "")
		queryParts := append(strings.SplitN(fragmentParts[0], "?", 2), "")
		return &uri{URL: url.URL{
			Scheme:   strings.ToLower(scheme),
			Opaque:   queryParts[0],
			RawQuery: queryParts[1],
			Fragment: fragmentParts[1],
		}}, nil
	}

	if runtime.GOOS == "windows" && len(scheme) == 1 {
		uriPath = scheme + ":" + filepath.ToSlash(uriPath)
		scheme = "file"
	}

	if strings.EqualFold(scheme, "file") {
		parsed, err := url.Parse(s)
		if err != nil {
			return nil, err
		}

		filePath := parsed.Path
		if parsed.Opaque != "" {
			filePath = parsed.Opaque
		}
		if len(parsed.Host) >= 2 && parsed.Host[1] == ':' {
			filePath = parsed.Host + parsed.Path
			parsed.Host = ""
		}
		if parsed.Host != "" {
			host := parsed.Hostname()
			if net.ParseIP(host) == nil && !rxHostName.MatchString(host) {
				return nil, errors.New("failed to validate host")
			}
			return &uri{URL: *parsed}, nil
		}
		if filePath == "" {
			return nil, errors.New("invalid file URI, path cannot be empty")
		}

		fileURI := NewFileURI(filePath).(*uri)
		fileURI.RawQuery = parsed.RawQuery
		fileURI.URL.Fragment = parsed.Fragment
		return fileURI, nil
	}

	scheme = strings.ToLower(scheme)
	if repo, err := ForScheme(scheme); err == nil {
		if custom, ok := repo.(CustomURIRepository); ok {
			return custom.ParseURI(s)
		}
	}

	parsed, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	parsed.Scheme = scheme

	if parsed.Host == "" {
		return &uri{URL: *parsed}, nil
	}

	host := parsed.Hostname()
	if net.ParseIP(host) == nil && !rxHostName.MatchString(host) {
		return nil, errors.New("failed to validate host")
	}
	return &uri{URL: *parsed}, nil
}
