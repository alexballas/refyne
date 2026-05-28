# Seekable storage reads design

Date: 2026-05-28
Status: Approved (pending spec review)

## Problem

`storage.Reader(uri)` returns a `fyne.URIReadCloser`, which is only an
`io.ReadCloser`. Consumers that need random access — most notably
`http.ServeContent`, which requires an `io.ReadSeeker` to satisfy HTTP range
requests — cannot seek.

Downstream (go2tv) works around this on Android by copying the entire backing
media file to a temporary file so it can be reopened as a seekable
`*os.File`. For a 4 GB MKV that means a full duplicate copy for every cast,
plus temp-file cleanup logic. The copy is unnecessary: the underlying data
sources can already provide seekable handles.

- Desktop `file://` reads already return a `*os.File` (seekable) — the
  capability is just not exposed through the interface.
- Android `content://` providers can return a real file descriptor via
  `ContentResolver.openFileDescriptor(uri, "r")` →
  `ParcelFileDescriptor.detachFd()`, which supports `lseek`.
- iOS `file://` URLs can be opened with `open(2)` for a real POSIX fd.

## Goal

Expose seekable reads through the public storage API when the backing
repository can provide them, and signal `ErrOperationNotSupported` otherwise so
callers can fall back to their own strategy (e.g. the temp-file copy). This
lets go2tv delete its Android temp-copy hack and the associated cleanup.

Non-goals:
- We do **not** add any temp-copy fallback inside Refyne. Fallback is the
  caller's responsibility.
- We do not change the existing `storage.Reader` behaviour or signature.

## Design

### 1. Public interface (`uri.go`)

A new extension interface parallel to `URIReadCloser`:

```go
// URIReadSeekCloser represents a seekable, cross-platform data stream from a
// file or data provider. It is the seekable counterpart of URIReadCloser and
// is suitable for consumers such as http.ServeContent that require random
// access.
//
// Since: 2.8
type URIReadSeekCloser interface {
	URIReadCloser
	io.Seeker
}
```

Effectively `io.ReadSeekCloser` + `URI()`.

### 2. Optional repository interface (`storage/repository/repository.go`)

Following the existing optional-extension pattern (`WritableRepository`,
`ListableRepository`, `CopyableRepository`, ...):

```go
// SeekableReadableRepository is an extension of the Repository interface for
// backends able to provide a seekable reader (for example, a real file
// descriptor). Repositories that cannot support seeking must not implement
// this interface; storage.ReaderSeeker will then report
// ErrOperationNotSupported.
//
// Since: 2.8
type SeekableReadableRepository interface {
	Repository

	// ReaderSeeker will be used to implement calls to storage.ReaderSeeker()
	// for the registered scheme of this repository.
	ReaderSeeker(u fyne.URI) (fyne.URIReadSeekCloser, error)
}
```

### 3. Storage entry point (`storage/uri.go`)

```go
// ReaderSeeker returns a URIReadSeekCloser set up to read from the resource
// that the URI references, when the backing repository supports seekable
// reads. Consumers that require a fallback (for example, copying the resource
// to a temporary seekable file) should handle a returned
// repository.ErrOperationNotSupported error.
//
// Each returned reader has an independent file offset; callers needing
// concurrent access (such as overlapping HTTP range requests) should open one
// reader per consumer.
//
// Since: 2.8
func ReaderSeeker(u fyne.URI) (fyne.URIReadSeekCloser, error) {
	repo, err := repository.ForURI(u)
	if err != nil {
		return nil, err
	}
	if seekRepo, ok := repo.(repository.SeekableReadableRepository); ok {
		return seekRepo.ReaderSeeker(u)
	}
	return nil, repository.ErrOperationNotSupported
}
```

Naming note: the original proposal called this `OpenSeeker`. We use
`ReaderSeeker` to mirror the existing `storage.Reader`. This was confirmed
during design.

### 4. Per-platform implementations

#### Desktop — `internal/repository/file.go`

`FileRepository.Reader` already returns a `*file` embedding `*os.File`, which
satisfies `io.Seeker`. Add:

```go
func (r *FileRepository) ReaderSeeker(u fyne.URI) (fyne.URIReadSeekCloser, error) {
	return openFile(u, false, false)
}
```

`openFile` already returns a `*file` embedding `*os.File`, so the result
satisfies `io.Seeker` with no further work. Add compile-time assertions:

```go
var _ repository.SeekableReadableRepository = (*FileRepository)(nil)
```

and update the existing `var _ fyne.URIReadCloser = (*file)(nil)` block to also
assert `var _ fyne.URIReadSeekCloser = (*file)(nil)`.

#### Mobile common — `internal/driver/mobile/file.go` and `repository.go`

A shared wrapper that adds `URI()` to a platform-provided
`io.ReadSeekCloser`:

```go
type fileOpenSeeker struct {
	io.ReadSeekCloser
	uri fyne.URI
}

func (f *fileOpenSeeker) URI() fyne.URI { return f.uri }

func seekableFileReaderForURI(u fyne.URI) (fyne.URIReadSeekCloser, error) {
	rsc, err := nativeFileOpenSeeker(&fileOpen{uri: u})
	if err != nil {
		return nil, err
	}
	if rsc == nil {
		return nil, repository.ErrOperationNotSupported
	}
	return &fileOpenSeeker{ReadSeekCloser: rsc, uri: u}, nil
}
```

In `repository.go`:

```go
func (m *mobileFileRepo) ReaderSeeker(u fyne.URI) (fyne.URIReadSeekCloser, error) {
	return seekableFileReaderForURI(u)
}

var _ repository.SeekableReadableRepository = (*mobileFileRepo)(nil)
```

`nativeFileOpenSeeker(*fileOpen) (io.ReadSeekCloser, error)` is implemented per
build tag.

#### Android — `internal/driver/mobile/file_android.go` and `android.c`

New C function (mirrors `openStream`, but obtains and detaches a fd):

```c
// Returns a detached, owned POSIX file descriptor for the content URI, or -1
// if the provider cannot supply one (exception, null, or streaming-only).
int openFileDescriptor(uintptr_t jni_env, uintptr_t ctx, char* uriCstr) {
	JNIEnv *env = (JNIEnv*)jni_env;
	jobject resolver = getContentResolver(jni_env, ctx);

	jclass resolverClass = (*env)->GetObjectClass(env, resolver);
	jmethodID openFd = find_method(env, resolverClass, "openFileDescriptor",
		"(Landroid/net/Uri;Ljava/lang/String;)Landroid/os/ParcelFileDescriptor;");

	jobject uri = parseURI(jni_env, ctx, uriCstr);
	jstring mode = (*env)->NewStringUTF(env, "r");
	jobject pfd = (*env)->CallObjectMethod(env, resolver, openFd, uri, mode);

	jthrowable err = (*env)->ExceptionOccurred(env);
	if (err != NULL) {
		(*env)->ExceptionClear(env);
		return -1;
	}
	if (pfd == NULL) {
		return -1;
	}

	jclass pfdClass = (*env)->GetObjectClass(env, pfd);
	jmethodID detachFd = find_method(env, pfdClass, "detachFd", "()I");
	jint fd = (*env)->CallIntMethod(env, pfd, detachFd);

	err = (*env)->ExceptionOccurred(env);
	if (err != NULL) {
		(*env)->ExceptionClear(env);
		return -1;
	}
	return (int)fd;
}
```

cgo declaration added to the `file_android.go` preamble:

```c
int openFileDescriptor(uintptr_t jni_env, uintptr_t ctx, char* uriCstr);
```

Go:

```go
func openFileDescriptor(uri string) int {
	uriStr := C.CString(uri)
	defer C.free(unsafe.Pointer(uriStr))

	fd := -1
	app.RunOnJVM(func(_, env, ctx uintptr) error {
		fd = int(C.openFileDescriptor(C.uintptr_t(env), C.uintptr_t(ctx), uriStr))
		return nil
	})
	return fd
}

func nativeFileOpenSeeker(f *fileOpen) (io.ReadSeekCloser, error) {
	if f.uri == nil || f.uri.String() == "" {
		return nil, nil
	}

	fd := openFileDescriptor(f.uri.String())
	if fd < 0 {
		return nil, repository.ErrOperationNotSupported
	}

	file := os.NewFile(uintptr(fd), f.uri.Name())
	if file == nil {
		return nil, repository.ErrOperationNotSupported
	}

	// Pipe-backed providers (openPipeHelper) hand out a non-seekable fd;
	// probe before promising seekability so the caller can fall back.
	if _, err := file.Seek(0, io.SeekCurrent); err != nil {
		file.Close()
		return nil, repository.ErrOperationNotSupported
	}

	return file, nil
}
```

JNI correctness notes (must be honoured in implementation):

1. **Use `detachFd()`, not `getFd()`.** `detachFd` transfers fd ownership out
   of the `ParcelFileDescriptor` to native code, so the fd stays valid after
   the local `pfd` reference is reclaimed when `RunOnJVM` returns. We then own
   the fd and must `Close()` it (`os.File.Close` does). `getFd()` keeps the PFD
   as owner, producing a use-after-free once the PFD is GC'd.
2. **Check `ExceptionOccurred`/`ExceptionClear` after each JNI call.**
   `openFileDescriptor` throws `FileNotFoundException` for providers that do
   not support `"r"` mode; an uncleared exception poisons the next JNI call.
3. **No global ref needed.** We return a plain `int`; unlike `openStream`,
   there is no Java object to keep alive across calls and nothing to
   `DeleteGlobalRef` on close.
4. **Use `openFileDescriptor`, not `openAssetFileDescriptor`.**
   `openAssetFileDescriptor` may return a sub-range (`startOffset` /
   `declaredLength`) of a possibly-compressed asset, so fd offset 0 would not
   be the data start. `openFileDescriptor` returns a whole-file PFD whose
   offset 0 is the data start, mapping 1:1 onto `os.File` /
   `http.ServeContent` semantics.
5. **Seekability probe.** Even with `openFileDescriptor`, a streaming provider
   can return a pipe fd where `lseek` returns `ESPIPE`. The
   `Seek(0, io.SeekCurrent)` probe detects this and returns
   `ErrOperationNotSupported`, so we never hand out a Seeker that fails
   mid-range-request.

#### iOS — `internal/driver/mobile/file_ios.go` and `file_ios.m`

iOS storage registers only the `file` scheme; URLs are security-scoped.
Open a real POSIX fd, mirroring Android.

ObjC:

```objc
// Returns a read-only POSIX fd for the file URL, or -1 on failure.
int iosOpenFileDescriptor(void* urlPtr) {
    NSURL* url = (NSURL*)urlPtr;
    BOOL scoped = [url startAccessingSecurityScopedResource];
    int fd = open([url fileSystemRepresentation], O_RDONLY);
    if (scoped) [url stopAccessingSecurityScopedResource];
    return fd;
}
```

cgo declaration added to the `file_ios.go` preamble:

```c
int iosOpenFileDescriptor(void* url);
```

Go (structurally identical to Android):

```go
func nativeFileOpenSeeker(f *fileOpen) (io.ReadSeekCloser, error) {
	if f.uri == nil || f.uri.String() == "" {
		return nil, nil
	}

	cStr := C.CString(f.uri.String())
	defer C.free(unsafe.Pointer(cStr))

	url := C.iosParseUrl(cStr)
	fd := int(C.iosOpenFileDescriptor(url))
	if fd < 0 {
		return nil, repository.ErrOperationNotSupported
	}

	file := os.NewFile(uintptr(fd), f.uri.Name())
	if file == nil {
		return nil, repository.ErrOperationNotSupported
	}
	if _, err := file.Seek(0, io.SeekCurrent); err != nil {
		file.Close()
		return nil, repository.ErrOperationNotSupported
	}
	return file, nil
}
```

iOS-specific note: `startAccessingSecurityScopedResource` gates the `open`,
not subsequent reads. Reads go through the kernel fd, not the `NSURL`, so the
balanced start → `open()` → stop sequence leaves a valid fd for the whole
session with no retained `NSURL`, no leak, and no custom close wrapper.
`#import` of `<fcntl.h>` is required for `open`/`O_RDONLY`.

#### Desktop build of the mobile driver — `internal/driver/mobile/file_desktop.go`

This build (`!ios && !android`) delegates `file://` to the internal
`FileRepository`, so `mobileFileRepo` is not registered. A stub is still needed
so the package compiles:

```go
func nativeFileOpenSeeker(*fileOpen) (io.ReadSeekCloser, error) {
	// no-op as we use the internal FileRepository
	return nil, nil
}
```

(`seekableFileReaderForURI` maps a nil reader to `ErrOperationNotSupported`.)

## Data flow

```
caller
  └─ storage.ReaderSeeker(u)
       └─ repository.ForURI(u)            -> repo
            └─ repo.(SeekableReadableRepository)?
                 ├─ yes -> repo.ReaderSeeker(u) -> URIReadSeekCloser
                 │            desktop:  *os.File-backed *file
                 │            android:  os.File over detached content:// fd
                 │            ios:      os.File over open()'d file:// fd
                 └─ no  -> ErrOperationNotSupported  (caller falls back)
```

## Error handling

- Unsupported scheme/repository: `repository.ErrOperationNotSupported`.
- Provider cannot supply a seekable fd (exception, null PFD, or pipe-backed
  stream): `ErrOperationNotSupported` (after closing any opened fd).
- `repository.ForURI` failure: propagated unchanged.
- Missing resource: existing repository errors (e.g. `storage.ErrNotExists`)
  propagate.

## Testing

Unit tests (run on desktop CI):

- `storage.ReaderSeeker` on a temp file: read from start, `Seek` to mid-file
  and read, `SeekEnd`, verify bytes; confirm offset independence between two
  readers of the same URI.
- `storage.ReaderSeeker` returns `ErrOperationNotSupported` for a repository
  that does not implement `SeekableReadableRepository` (e.g. the in-memory
  repository).
- `internal/repository.FileRepository.ReaderSeeker` repository-level test.
- Compile-time interface assertions (`var _ ...`) for `FileRepository`,
  `mobileFileRepo`, and the `*file` / `fileOpenSeeker` reader types.

Platform limits:

- Android and iOS paths are validated by build (cgo compiles) and verified
  manually on-device, since CI cannot exercise `ContentResolver` /
  security-scoped `NSURL`s. This limitation is accepted.

## Downstream impact (go2tv, informational)

Replace the temp-copy + cleanup (`internal/gui/actions_mobile.go` and the
cleanup in `stopAction` / `gui_mobile.go`) with:

```go
rs, err := storage.ReaderSeeker(uri)
if errors.Is(err, repository.ErrOperationNotSupported) {
    // existing copy-to-temp fallback
} else if err != nil {
    // handle
} else {
    defer rs.Close()
    http.ServeContent(w, r, uri.Name(), modTime, rs)
}
```

## Files touched (Refyne)

- `uri.go` — add `URIReadSeekCloser`.
- `storage/repository/repository.go` — add `SeekableReadableRepository`.
- `storage/uri.go` — add `ReaderSeeker`.
- `internal/repository/file.go` — implement `ReaderSeeker`, add assertions.
- `internal/driver/mobile/file.go` — add `fileOpenSeeker`,
  `seekableFileReaderForURI`.
- `internal/driver/mobile/repository.go` — add `ReaderSeeker`, add assertion.
- `internal/driver/mobile/file_android.go` — `nativeFileOpenSeeker`,
  `openFileDescriptor` wrapper + cgo decl.
- `internal/driver/mobile/android.c` — `openFileDescriptor` C function.
- `internal/driver/mobile/file_ios.go` — `nativeFileOpenSeeker` + cgo decl.
- `internal/driver/mobile/file_ios.m` — `iosOpenFileDescriptor`.
- `internal/driver/mobile/file_desktop.go` — `nativeFileOpenSeeker` stub.
- `CHANGELOG.md` — note the new API (2.8).
