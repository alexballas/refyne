package metadata

// FyneApp describes the top level metadata for building a fyne application
type FyneApp struct {
	Website      string `toml:",omitempty"`
	Description  string `toml:",omitempty"`
	Details      AppDetails
	AdaptiveIcon *AdaptiveIcon `toml:",omitempty"`

	Development map[string]string `toml:",omitempty"`
	Release     map[string]string `toml:",omitempty"`
	Source      *AppSource        `toml:",omitempty"`
	CanOpen     *CanOpen          `toml:",omitempty"`
	LinuxAndBSD *LinuxAndBSD      `toml:",omitempty"`
	Android     *Android          `toml:",omitempty"`
	Languages   []string          `toml:",omitempty"`
	Migrations  map[string]bool   `toml:",omitempty"`
}

// AppDetails describes the build information, this group may be OS or arch specific
type AppDetails struct {
	Icon     string `toml:",omitempty"`
	Name, ID string `toml:",omitempty"`
	Version  string `toml:",omitempty"`
	Build    int    `toml:",omitempty"`
}

type AdaptiveIcon struct {
	Foreground string `toml:",omitempty"`
	Background string `toml:",omitempty"`
	Monochrome string `toml:",omitempty"`
}

type AppSource struct {
	Repo, Dir string `toml:",omitempty"`
}

// LinuxAndBSD describes specific metadata for desktop files on Linux and BSD.
type LinuxAndBSD struct {
	GenericName string   `toml:",omitempty"`
	Categories  []string `toml:",omitempty"`
	Comment     string   `toml:",omitempty"`
	Keywords    []string `toml:",omitempty"`
	ExecParams  string   `toml:",omitempty"`
}

// CanOpen represents a selection of file types (mime etc) that this application can open.
type CanOpen struct {
	MimeTypes string `toml:",omitempty"`
}

// Android describes Android specific packaging metadata.
//
// Since: 2.8
type Android struct {
	// ShareMimeTypes registers ACTION_SEND / ACTION_VIEW intent filters for these
	// MIME types, making the app a share target and an "open with" handler.
	// Values are Android MIME patterns such as "video/*" or "image/png".
	ShareMimeTypes []string `toml:",omitempty"`

	// BackgroundService configures the foreground service started by
	// driver/mobile.BackgroundSession. Android requires every foreground service
	// to declare the category of work it performs.
	//
	// Since: 2.8
	BackgroundService *AndroidBackgroundService `toml:",omitempty"`

	// MulticastDiscovery requests CHANGE_WIFI_MULTICAST_STATE so
	// driver/mobile.MulticastLocker can let multicast and broadcast packets
	// through the Wi-Fi filter. Needed for SSDP and mDNS device discovery.
	//
	// Since: 2.8
	MulticastDiscovery bool `toml:",omitempty"`

	// BatteryOptimizationExemption requests REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
	// which lets driver/mobile.BatteryOptimization show the system dialog that
	// exempts the app in one tap. Google Play restricts this permission to a
	// short list of app categories, so it is opt-in separately from
	// BackgroundService; without it the exemption prompt still works but
	// drops the user on the system settings list to find the app by hand.
	//
	// Since: 2.8
	BatteryOptimizationExemption bool `toml:",omitempty"`
}

// AndroidBackgroundService describes the Android foreground service used for a
// background session.
//
// Since: 2.8
type AndroidBackgroundService struct {
	// Type identifies the work performed by the service. Supported values are
	// "mediaPlayback" and "connectedDevice".
	Type AndroidBackgroundServiceType

	// KeepCPUAwake holds a partial wake lock while the session is active. Enable
	// it only when work must continue while the screen is off.
	KeepCPUAwake bool `toml:",omitempty"`

	// KeepWiFiAwake asks Android to keep an active Wi-Fi connection awake while
	// the session is active. Enable it only for continuous Wi-Fi traffic.
	KeepWiFiAwake bool `toml:",omitempty"`
}

// AndroidBackgroundServiceType identifies an Android foreground-service use
// case.
//
// Since: 2.8
type AndroidBackgroundServiceType string

const (
	// AndroidBackgroundServiceMediaPlayback continues audio or video playback
	// while the app is backgrounded.
	AndroidBackgroundServiceMediaPlayback AndroidBackgroundServiceType = "mediaPlayback"

	// AndroidBackgroundServiceConnectedDevice maintains an interaction with an
	// external device over a network or other connection.
	AndroidBackgroundServiceConnectedDevice AndroidBackgroundServiceType = "connectedDevice"
)
