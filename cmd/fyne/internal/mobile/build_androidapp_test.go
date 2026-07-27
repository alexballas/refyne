package mobile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexballas/refyne/v2/cmd/fyne/internal/metadata"
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

func TestManifestAndroidFeaturesAreIndependent(t *testing.T) {
	const (
		fgsService        = `android:name="org.golang.app.FyneForegroundService"`
		fgsMediaType      = `android:foregroundServiceType="mediaPlayback"`
		fgsConnectedType  = `android:foregroundServiceType="connectedDevice"`
		keepCPU           = "org.fyne.background.KEEP_CPU_AWAKE"
		keepWiFi          = "org.fyne.background.KEEP_WIFI_AWAKE"
		permFGS           = "android.permission.FOREGROUND_SERVICE"
		permFGSMedia      = "android.permission.FOREGROUND_SERVICE_MEDIA_PLAYBACK"
		permFGSConnected  = "android.permission.FOREGROUND_SERVICE_CONNECTED_DEVICE"
		permChangeNetwork = "android.permission.CHANGE_NETWORK_STATE"
		permNotify        = "android.permission.POST_NOTIFICATIONS"
		permWake          = "android.permission.WAKE_LOCK"
		permMulti         = "android.permission.CHANGE_WIFI_MULTICAST_STATE"
		permBattery       = "android.permission.REQUEST_IGNORE_BATTERY_OPTIMIZATIONS"
	)

	base := manifestTmplData{JavaPkgPath: "com.example.app", Name: "Example", LibName: "example"}

	tt := []struct {
		name string
		set  func(*manifestTmplData)
		want []string
	}{
		{
			name: "no opt-in",
			set:  func(*manifestTmplData) {},
		},
		{
			name: "media playback",
			set: func(d *manifestTmplData) {
				d.BackgroundService = &backgroundServiceTmplData{
					Type:       "mediaPlayback",
					Permission: permFGSMedia,
				}
			},
			want: []string{fgsService, fgsMediaType, permFGS, permFGSMedia, permNotify},
		},
		{
			name: "media playback with CPU lock",
			set: func(d *manifestTmplData) {
				d.BackgroundService = &backgroundServiceTmplData{
					Type:                    "mediaPlayback",
					Permission:              permFGSMedia,
					KeepCPUAwake:            true,
					NeedsWakeLockPermission: true,
				}
			},
			want: []string{fgsService, fgsMediaType, keepCPU, permFGS, permFGSMedia, permNotify, permWake},
		},
		{
			name: "media playback with Wi-Fi lock",
			set: func(d *manifestTmplData) {
				d.BackgroundService = &backgroundServiceTmplData{
					Type:                    "mediaPlayback",
					Permission:              permFGSMedia,
					KeepWiFiAwake:           true,
					NeedsWakeLockPermission: true,
				}
			},
			want: []string{fgsService, fgsMediaType, keepWiFi, permFGS, permFGSMedia, permNotify, permWake},
		},
		{
			name: "connected device",
			set: func(d *manifestTmplData) {
				d.BackgroundService = &backgroundServiceTmplData{
					Type:                   "connectedDevice",
					Permission:             permFGSConnected,
					PrerequisitePermission: permChangeNetwork,
				}
			},
			want: []string{
				fgsService, fgsConnectedType, permFGS, permFGSConnected,
				permChangeNetwork, permNotify,
			},
		},
		{
			name: "MulticastDiscovery",
			set:  func(d *manifestTmplData) { d.MulticastDiscovery = true },
			want: []string{permMulti},
		},
		{
			name: "BatteryOptimizationExemption",
			set:  func(d *manifestTmplData) { d.BatteryOptimizationExemption = true },
			want: []string{permBattery},
		},
	}

	all := []string{
		fgsService, fgsMediaType, fgsConnectedType, keepCPU, keepWiFi,
		permFGS, permFGSMedia, permFGSConnected, permChangeNetwork,
		permNotify, permWake, permMulti, permBattery,
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			data := base
			tc.set(&data)
			got := renderManifest(t, data)

			wanted := make(map[string]bool, len(tc.want))
			for _, w := range tc.want {
				wanted[w] = true
				if !strings.Contains(got, w) {
					t.Errorf("manifest missing %q:\n%s", w, got)
				}
			}
			for _, decl := range all {
				if !wanted[decl] && strings.Contains(got, decl) {
					t.Errorf("manifest leaked %q that this key does not own:\n%s", decl, got)
				}
			}
		})
	}
}

func TestManifestForegroundServiceIsInsideApplication(t *testing.T) {
	for _, serviceType := range []string{"mediaPlayback", "connectedDevice"} {
		got := renderManifest(t, manifestTmplData{
			JavaPkgPath: "com.example.app", Name: "Example", LibName: "example",
			BackgroundService: &backgroundServiceTmplData{Type: serviceType},
		})

		service := strings.Index(got, "org.golang.app.FyneForegroundService")
		appEnd := strings.Index(got, "</application>")
		if service < 0 || appEnd < 0 || service > appEnd {
			t.Errorf("%s service is not inside <application>:\n%s", serviceType, got)
		}
	}
}

func TestBackgroundServiceAlwaysRequiresAAPT2(t *testing.T) {
	if requiresAAPT2(false, nil) {
		t.Error("plain legacy manifest unexpectedly requires aapt2")
	}
	if !requiresAAPT2(true, nil) {
		t.Error("adaptive icon should require aapt2")
	}
	if !requiresAAPT2(false, &backgroundServiceTmplData{}) {
		t.Error("foreground service should require aapt2 without an adaptive icon")
	}
}

func TestCompileBackgroundServiceWithoutAdaptiveIcon(t *testing.T) {
	if os.Getenv("ANDROID_HOME") == "" {
		t.Skip("ANDROID_HOME is not configured")
	}

	legacyIcon := filepath.Join("..", "..", "..", "..", "dialog", "testdata", "test.png")
	for _, tc := range []struct {
		name string
		icon string
	}{
		{name: "without icon"},
		{name: "with legacy icon", icon: legacyIcon},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := renderManifest(t, manifestTmplData{
				JavaPkgPath: "com.example.app",
				Name:        "Example",
				LibName:     "example",
				LegacyIcon:  tc.icon != "",
				BackgroundService: &backgroundServiceTmplData{
					Type:       "mediaPlayback",
					Permission: "android.permission.FOREGROUND_SERVICE_MEDIA_PLAYBACK",
				},
			})
			_, _, compiledManifest, err := compileAndroidResources(
				t.TempDir(), []byte(manifest), tc.icon, "", "", "", 35, 1, "1.0",
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(compiledManifest); err != nil {
				t.Fatalf("compiled manifest is unavailable: %v", err)
			}
		})
	}
}

func TestBackgroundServiceManifestData(t *testing.T) {
	androidMeta := &metadata.Android{
		BackgroundService: &metadata.AndroidBackgroundService{
			Type:          metadata.AndroidBackgroundServiceConnectedDevice,
			KeepCPUAwake:  true,
			KeepWiFiAwake: true,
		},
	}

	got, err := backgroundServiceManifestData(androidMeta)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "connectedDevice" ||
		got.Permission != "android.permission.FOREGROUND_SERVICE_CONNECTED_DEVICE" ||
		got.PrerequisitePermission != "android.permission.CHANGE_NETWORK_STATE" ||
		!got.NeedsWakeLockPermission {
		t.Fatalf("unexpected manifest data: %#v", got)
	}

	androidMeta.BackgroundService.Type = "unsupported"
	if _, err := backgroundServiceManifestData(androidMeta); err == nil {
		t.Error("unsupported service type was accepted")
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
