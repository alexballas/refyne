# Handoff

Repo: `/home/alex/test/refyne`
Current branch: `main` at original starting point `d281c2e6b`

Important local ref created during the earlier audit:

`refs/remotes/upstream-latest/develop` fetched from `https://github.com/fyne-io/fyne.git refs/heads/develop`.

## Original Request

User asked: "I need to understand what new was added to the fyne (upstream) develop branch. Make a list of all the features we're currently lack compared to that. Rank them"

The follow-up question was: "Which one should I pick?" The recommendation was to start with **core app cache + scheduled notifications**.

## Comparison Facts

- Current Refyne before this work: `d281c2e6bc17a3dd47b03372f4ff20c517f2e6e3`
- Upstream Fyne develop at audit time: `005a376f35362d48fbd590273f084ae2fb548538`
- Existing local `upstream/develop` was stale at `4c0c29f7`
- Merge base: `c6356eff6ac7c13063c19e611cb1d1fcc26211e8`
- Divergence at audit time: Refyne was `115` commits ahead and `633` behind upstream develop.

## Progress On Item 1

Implementation of **core app cache + scheduled notifications** has started and is mostly complete in the worktree.

Implemented:

- Added public `fyne.Cache` interface in `cache.go`.
- Extended `fyne.App` with `Cache()`, `ScheduleNotification`, and `CancelScheduledNotification`.
- Added `fyne.ScheduledNotification`, `NewScheduledNotification`, `ID()`, and `Cancel()`.
- Added `CacheResourceFromURLString`.
- Added app cache implementation under `app/cache*.go`.
- Wired `fyneApp` to create the cache and start a persistent fallback scheduler.
- Added `internal/scheduler` with cache-backed JSON persistence, restart restoration, cancellation, and due-on-start delivery.
- Added fallback scheduled notification support for linux/xdg, wasm, unknown OS, and mobile simulator paths.
- Added native scheduled notification hooks for macOS and Windows.
- Added Android scheduling bridge:
  - Go/C bridge in `app/app_mobile_and.go` and `app/app_mobile_and.c`.
  - Java scheduling methods in `internal/driver/mobile/app/GoNativeActivity.java`.
  - New `internal/driver/mobile/app/FyneNotificationReceiver.java`.
  - Android manifest receiver declaration in `cmd/fyne/internal/templates/data/AndroidManifest.xml`.
  - `cmd/fyne/internal/mobile/gendex/gendex.go` now reads Java sources from this repo and packages all generated `org/golang/app/*.class` files.
  - Regenerated `cmd/fyne/internal/mobile/dex.go`.
- Updated test app support with an in-memory cache and scheduled notification assertions.
- Added cache and scheduler tests.

## Progress On Form Validation APIs

Implementation of **form validation APIs** is complete in the worktree.

Implemented:

- Added public `fyne.Requireable` interface in `validation.go`.
- Added `Entry.HasValue()` and `Entry.SetOnRequiredChanged(...)`.
- Wired `Entry` to notify required-state changes when text transitions between empty and non-empty.
- Added `FormItem.Required`.
- Added `Form.Validator` with submit-button gating and form-level validation helper text.
- Added `Form.RemoveItem`.
- Added form required-state handling for `Requireable` widgets and logging for unsupported required widgets.
- Added widget tests for required entries, required form items, form-level validation, and item removal.

## Progress On RichText/Markdown/Text APIs

Implementation of **RichText/Markdown/text** parity has been completed in the worktree.

Implemented:

- Added public `fyne.TextStyle.Strikethrough`.
- Added strikethrough/underline canvas text decoration rendering for software and GL painters.
- Added markdown GFM extension support for strikethrough, task lists, and tables.
- Added markdown autolink support for angle-bracket URLs.
- Added markdown HTML entity decoding for text segments.
- Added rich text `CodeBlockSegment` with a scrollable monospace panel.
- Added inline-code background rendering while preserving text-like wrapping/alignment behavior.
- Added rich text `CheckBoxSegment` for task list items.
- Added rich text `TableSegment` with header/body cells, per-column alignment, and inline formatting preservation.
- Improved markdown heading rendering so headings can contain links, emphasis, inline code, and strikethrough.
- Improved nested blockquote and nested list parsing/layout, including quote/list indentation metadata.
- Added wrapped hyperlink sibling hover synchronization.
- Added focused tests for markdown blockquotes, nested lists, task lists, tables, autolinks, code blocks, inline code, strikethrough emphasis, and heading/link combinations.

## Progress On Canvas Graphics / Shaders

Implementation of **canvas graphics / effects** (originally listed below as remaining item 1) is largely complete in the worktree. This area was previously undocumented in this handoff even though it is part of the diff.

Implemented:

- Added public canvas types: `canvas.Blur`, `canvas.Shader`, `canvas.Shadow`, `canvas.Ellipse`, `canvas.RegularPolygon`, `canvas.ArbitraryPolygon`.
- Added `canvas.NewShader` / `canvas.NewShaderAnimation` and a `Shadow` field on `canvas.Rectangle`, `canvas.Circle`, and `canvas.Ellipse`.
- GL painter: `drawBlur`, `drawShader` (with lazy per-shader program cache and texture binding), `drawEllipse`, `drawRegularPolygon`, `drawArbitraryPolygon`, plus shadow rendering for rectangle/round-rectangle/circle/ellipse via new shader uniforms (`add_shadow`, `shadow_*`).
- Software painter: `drawBlur` and `drawShadow` (gaussian via `github.com/anthonynsimon/bild`), `drawEllipse`, `drawRegularPolygon`, `drawArbitraryPolygon`; shadows for rectangle/circle/ellipse.
- Added shaders `blur`, `ellipse`, `regular_polygon`, `arbitrary_polygon` (desktop + `_es` variants), extended `rectangle`/`round_rectangle` shaders with shadow uniforms, and wired them into `shaders.go` / `shaders_es.go`.
- Added new GL backend entry points (`CopyTexSubImage2D`, `SetUniform1fv`, `SetUniform2fv`, etc.) symmetrically across `gl_core.go`, `gl_es.go`, `gl_gomobile.go`, `gl_wasm.go`.
- Added blur-kernel cache in `internal/cache/blur_kernel.go` with expiry cleanup in `internal/cache/base.go`.
- Extended `internal/painter/vector.go` `VectorPad` for the new shapes.
- Added new dependency `github.com/anthonynsimon/bild` for software-painter blur/shadow.
- Added canvas tests for blur, ellipse, regular/arbitrary polygon, shader, and shadow.

## Progress On Accessibility

Implementation of **accessibility** (`fyne.Accessible`, roles, widget labels/roles, opt-in native bridges behind the `accessibility` build tag) is complete in the worktree.

Implemented (core, unconditional):

- Added public `fyne.AccessibleRole` type with `AccessibleRoleButton`, `AccessibleRoleContainer`, `AccessibleRoleLink`, `AccessibleRoleText` constants and the `fyne.Accessible` interface (`AccessibilityLabel()`, `AccessibilityRole()`) in `accessibility.go`.
- `*fyne.Container` now conforms to `Accessible` (label `"Container"`, role `AccessibleRoleContainer`).
- Widget roles/labels: `widget.Button` (text or icon name / `AccessibleRoleButton`), `widget.Label` (text / `AccessibleRoleText`), `widget.Hyperlink` (text / `AccessibleRoleLink`).

Implemented (driver wiring, unconditional calls into build-tag-gated methods):

- glfw: `updateAccessibility()` after repaint in `loop.go`, on `Show` and `SetContent` in `window.go`; `cleanupAccessibilityForWindow()` in `Close` (before `w.closing = true` so `view()` is still valid); `initAccessibilityForWindow()` at the end of `create()` in `window_desktop.go`.
- mobile: `updateAccessibility()` after publish in `driver.go` `handlePaint`; `initAccessibilityForWindow()` in `window.go` `Show`; `cleanupAccessibilityForWindow()` in `window.go` `Close`.

Implemented (native bridges, behind the `accessibility` build tag):

- glfw macOS: `accessibility_darwin.go` + `.h`/`.m` (NSAccessibility element tree, recursive parent/child model).
- glfw Windows: `accessibility_windows.go` + `.h`/`.c` (flat element model, leaf-only).
- glfw stub: `accessibility_notdarwin.go` (`!accessibility || (!darwin && !windows)`).
- mobile Android: `accessibility_android.go` + `.c` (JNI bridge, flat virtual-view overlay).
- mobile iOS: `accessibility_ios.go` + `.m` (UIAccessibility elements).
- mobile stub: `accessibility_notandroid.go` (`!accessibility || (!android && !ios)`).
- Android Java side: added accessibility methods/fields to `internal/driver/mobile/app/GoNativeActivity.java` (`setupAccessibility`, `doSetupAccessibility`, `clearAccessibilityNodes`, `addAccessibilityNode`, `commitAccessibilityNodes`, `applySnapshot`, `rebuildA11yViews`, `mA11yContainer` real-view overlay, `ROLE_*` constants). Only the accessibility additions were ported; upstream's unrelated refactors in that file (hoisted IME listeners, system-bar appearance) were intentionally not pulled in.

Notes:

- Native bridge `.go` files were copied verbatim from upstream with the import path rewritten from `fyne.io/fyne/v2` to `github.com/alexballas/refyne/v2`.
- Build-tag coverage is complete and non-overlapping: every `*window` gets `updateAccessibility`/`initAccessibilityForWindow`/`cleanupAccessibilityForWindow` from exactly one file for any (tag, GOOS) combination, including wasm (resolves to the glfw `notdarwin` stub).
- Regenerated the embedded Android dex; decoded `dex.go` now contains `setupAccessibility`, `addAccessibilityNode`, `commitAccessibilityNodes`, `clearAccessibilityNodes`, `doSetupAccessibility`, `rebuildA11yViews` (and still retains the scheduled-notification symbols).
- Added tests: `accessibility_test.go` (container) and `widget/accessibility_test.go` (button/label/hyperlink labels and roles). Upstream ships no accessibility tests.

Verification for accessibility passed:

```sh
gofumpt -l -w .
git diff --check
go build ./...
go build -tags accessibility ./...
go test ./ ./widget ./theme ./container ./test
# native bridges cross-compile with the accessibility tag:
ANDROID_HOME=/home/alex/Android/Sdk ANDROID_NDK_HOME=/home/alex/Downloads/android-ndk-r27d CGO_ENABLED=1 GOOS=android GOARCH=arm64 \
  CC=$NDK/.../aarch64-linux-android35-clang CXX=$NDK/.../aarch64-linux-android35-clang++ \
  go build -tags accessibility ./internal/driver/mobile/
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -tags accessibility ./internal/driver/glfw/
ANDROID_HOME=/home/alex/Android/Sdk go generate ./cmd/fyne/internal/mobile
go build ./cmd/fyne/...
```

Caveat: the macOS (`accessibility_darwin.*`) and iOS (`accessibility_ios.*`) native bridges could not be compiled in this Linux environment (no Apple toolchain). Their Go-side API usage is identical to the Windows/Android bridges that *were* cross-compiled successfully, and the source is verbatim-from-upstream plus the import-path rewrite, so risk is low — but they still need a real darwin/ios build to confirm the cgo layer.

## Post-Port Fix And Cleanup

After the port, an audit found and fixed one functional regression plus dead code:

- **Fixed GL text stretch (`internal/painter/gl/texture.go`)**: the text feature added `paint.TextVectorPad` to the GL text *quad* in `drawText` but the matching `newGlTextTexture` change (sizing the glyph image at the padded height) was missed, so every glyph texture was stretched ~1px onto a taller quad and rendered blurry. The software painter was unaffected (it draws into a taller transparent canvas), which is why tests/software renders did not catch it. `newGlTextTexture` now adds `paint.TextVectorPad` to the texture height, matching upstream.
- Removed orphaned shader files `internal/painter/gl/shaders/polygon.frag` and `polygon_es.frag` (the deprecated `canvas.Polygon` now compiles from `regular_polygon.frag`).
- Removed dead GL helpers `vecSquareCoords` and `vecRectCoordsWithPad` (orphaned by the shadow refactor) and the long-unused GL enum constants `constantAlpha`, `oneMinusConstantAlpha`, `noBuffer` (removed from all four backends for symmetry).

Audit confirmed the canvas-graphics/shader port is otherwise complete: all `canvas.*` types are dispatched, every shader has a desktop and `_es` variant that resolves in both name switches, all four GL backends create the same program set, and `internal/painter/gl` + `internal/driver/mobile` cross-compile for desktop, wasm, and android/arm64.

## Android Regeneration

User provided:

```sh
ANDROID_HOME=/home/alex/Android/Sdk
ANDROID_NDK_HOME=/home/alex/Downloads/android-ndk-r27d
```

Regenerated embedded Android dex with:

```sh
ANDROID_HOME=/home/alex/Android/Sdk go generate ./cmd/fyne/internal/mobile
```

Verified decoded `dex.go` contains:

- `FyneNotificationReceiver`
- `scheduleNotification`
- `cancelScheduledNotification`
- `AlarmManager`

## Verification Run

Passed:

```sh
gofumpt -l -w .
git diff --check
ANDROID_HOME=/home/alex/Android/Sdk go generate ./cmd/fyne/internal/mobile
go test ./cmd/fyne/internal/mobile ./cmd/fyne/internal/mobile/binres ./cmd/fyne/internal/templates ./cmd/fyne/internal/commands ./internal/scheduler ./app ./test ./theme
go test -tags ci ./app ./test ./internal/scheduler
go list ./... | rg -v '/internal/driver/(glfw|mobile)$' | xargs go test
```

Latest focused verification for form validation APIs passed:

```sh
gofumpt -l -w .
go test ./widget
go test ./dialog
git diff --check
```

Latest focused verification for RichText/Markdown/text passed:

```sh
gofumpt -l -w .
go test ./widget
go test ./internal/painter ./internal/painter/software ./test
go test ./canvas ./internal/painter/gl
git diff --check
```

Android cross-compile checks passed:

```sh
ANDROID_HOME=/home/alex/Android/Sdk ANDROID_NDK_HOME=/home/alex/Downloads/android-ndk-r27d CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC=/home/alex/Downloads/android-ndk-r27d/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android35-clang CXX=/home/alex/Downloads/android-ndk-r27d/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android35-clang++ go test -c -o /tmp/refyne-app-android-arm64.test ./app
ANDROID_HOME=/home/alex/Android/Sdk ANDROID_NDK_HOME=/home/alex/Downloads/android-ndk-r27d CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC=/home/alex/Downloads/android-ndk-r27d/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android35-clang CXX=/home/alex/Downloads/android-ndk-r27d/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android35-clang++ go test -c -o /tmp/refyne-mobile-app-android-arm64.test ./internal/driver/mobile/app
```

Known full-suite caveat:

- `go test ./...` still fails in existing packages unrelated to this feature:
  - `internal/driver/glfw`: `TestMenuBar` visual golden mismatch.
  - `internal/driver/mobile`: `Test_canvas_Dragged` assertions.

Generated failed golden PNGs were removed after the failed full-suite run.

## Current Worktree Notes

Implementation files are modified/untracked. `HANDOFF.md` is also untracked from the original handoff flow.

Important new files:

- `cache.go`
- `app/app_schedule.go`
- `app/cache.go`
- `app/cache_mobile.go`
- `app/cache_noos.go`
- `app/cache_other.go`
- `app/cache_test.go`
- `internal/scheduler/scheduler.go`
- `internal/scheduler/scheduler_test.go`
- `internal/driver/mobile/app/FyneNotificationReceiver.java`
- `test/memcache.go`
- `widget/richtext_codeinline_test.go`
- `canvas/blur.go`, `canvas/shader.go`, `canvas/shadow.go`, `canvas/ellipse.go`, `canvas/regularpolygon.go`, `canvas/arbitrary_polygon.go` (plus their `_test.go` files)
- `internal/cache/blur_kernel.go`
- `internal/painter/gl/shaders/{blur,blur_es}.{frag,vert}`, `internal/painter/gl/shaders/{ellipse,ellipse_es,regular_polygon,regular_polygon_es,arbitrary_polygon,arbitrary_polygon_es}.frag`
- `accessibility.go`, `accessibility_test.go`, `widget/accessibility_test.go`
- `internal/driver/glfw/accessibility_{darwin,notdarwin,windows}.go` (+ `accessibility_darwin.{h,m}`, `accessibility_windows.{h,c}`)
- `internal/driver/mobile/accessibility_{android,ios,notandroid}.go` (+ `accessibility_android.c`, `accessibility_ios.m`)

Important modified generated file:

- `cmd/fyne/internal/mobile/dex.go` (regenerated for both scheduled notifications and accessibility Java methods)

## Remaining Ranked Missing Feature Areas

1. ~~Canvas graphics/effects: `Blur`, `Shader`, `NewShaderAnimation`, `Shadow`, `Ellipse`, `ArbitraryPolygon`, `RegularPolygon`, shadows.~~ **Done** — see "Progress On Canvas Graphics / Shaders" above.
2. ~~Accessibility: `fyne.Accessible`, roles, widget labels/roles, opt-in native bridges behind the `accessibility` tag.~~ **Done** — see "Progress On Accessibility" above.
3. Desktop window controls: `desktop.Window`, secondary fullscreen, always-on-top, requested position, `HasSecondaryDisplay`, `IOSWindowContext`.
4. Theme/visual customization: new radius/blur/split/inner-window theme names, active/inactive inner windows, shadows/radius polish.
5. Collection widget API parity: `List.Bind`, `List.Unbind`, upstream `Highlight` methods on list/grid/table/tree. Refyne already has custom list/grid keyboard hooks not in upstream.
6. Mobile touch movement: `mobile.Movable`, per-finger `TouchMoved`.
7. CLI/tooling: `fyne get`, public deprecated `cmd/fyne/commands`, newer web/wasm packaging flow.
8. Public software canvas cleanup: `driver.Painter`, `driver/software.WindowlessCanvas`, painter constructors.
9. Test helpers beyond completed items as needed.

## Caveats

- Do not count Refyne-only additions as missing: seekable storage reads, Wayland client-side decorations, extra list/grid keyboard hooks.
- After future Go code changes, follow `AGENTS.md`: run `gofumpt -l -w .`.
- If upstream needs to be refreshed again, fetch `fyne-io/fyne` develop into `refs/remotes/upstream-latest/develop`.
