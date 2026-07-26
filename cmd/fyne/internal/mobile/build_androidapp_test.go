package mobile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alexballas/refyne/v2/cmd/fyne/internal/templates"
)

func renderManifest(t *testing.T, data manifestTmplData) string {
	t.Helper()

	buf := new(bytes.Buffer)
	if err := templates.ManifestAndroid.Execute(buf, data); err != nil {
		t.Fatalf("rendering manifest: %v", err)
	}
	return buf.String()
}

// An app that does not set Android.ShareMimeTypes must get exactly the manifest it
// got before intent filters existed - no launchMode, no extra filters, and no stray
// whitespace that could show up in a diff.
func TestManifestWithoutShareMimeTypes(t *testing.T) {
	base := manifestTmplData{JavaPkgPath: "com.example.app", Name: "Example", LibName: "example"}
	got := renderManifest(t, base)

	for _, unwanted := range []string{"launchMode", "android.intent.action.SEND", "android.intent.action.VIEW", "mimeType"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("manifest contains %q without ShareMimeTypes:\n%s", unwanted, got)
		}
	}

	const want = `		android:exported="true"
		android:theme="@android:style/Theme">`
	if !strings.Contains(got, want) {
		t.Errorf("activity attributes changed shape:\n%s", got)
	}

	const wantTail = `		</intent-filter>
	</activity>`
	if !strings.Contains(got, wantTail) {
		t.Errorf("activity body changed shape:\n%s", got)
	}
}

func TestManifestWithShareMimeTypes(t *testing.T) {
	base := manifestTmplData{
		JavaPkgPath:    "com.example.app",
		Name:           "Example",
		LibName:        "example",
		ShareMimeTypes: []string{"video/*", "audio/*"},
	}
	got := renderManifest(t, base)

	for _, want := range []string{
		`android:launchMode="singleTask"`,
		`<action android:name="android.intent.action.SEND" />`,
		`<action android:name="android.intent.action.VIEW" />`,
		`<data android:scheme="content" />`,
		`<data android:mimeType="video/*" />`,
		`<data android:mimeType="audio/*" />`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("manifest missing %q:\n%s", want, got)
		}
	}

	// ACTION_SEND intents carry no data URI, so a scheme on that filter would stop
	// it matching anything. Only the VIEW filter may declare one.
	send := got[strings.Index(got, "android.intent.action.SEND"):strings.Index(got, "android.intent.action.VIEW")]
	if strings.Contains(send, "android:scheme") {
		t.Errorf("SEND filter must not declare a scheme:\n%s", send)
	}

	if n := strings.Count(got, `<data android:mimeType="video/*" />`); n != 2 {
		t.Errorf("expected video/* in both filters, found %d", n)
	}
}
