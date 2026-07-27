// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mobile

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alexballas/refyne/v2/cmd/fyne/internal/metadata"
	"github.com/alexballas/refyne/v2/cmd/fyne/internal/mobile/binres"
	"github.com/alexballas/refyne/v2/cmd/fyne/internal/templates"
	"github.com/alexballas/refyne/v2/cmd/fyne/internal/util"
	"golang.org/x/tools/go/packages"
)

type manifestTmplData struct {
	JavaPkgPath  string
	Name         string
	Debug        bool
	LibName      string
	Version      string
	Build        int
	AdaptiveIcon bool
	LegacyIcon   bool

	// ShareMimeTypes, when non-empty, adds ACTION_SEND and ACTION_VIEW intent
	// filters for those types and switches the activity to singleTask so a share
	// re-uses the running instance instead of creating a second one.
	ShareMimeTypes []string

	// BackgroundService declares FyneForegroundService and the permissions for
	// its configured use case. MulticastDiscovery and
	// BatteryOptimizationExemption each add the single permission they name. An
	// app that configures none of them gets the manifest it always got.
	BackgroundService            *backgroundServiceTmplData
	MulticastDiscovery           bool
	BatteryOptimizationExemption bool
}

type backgroundServiceTmplData struct {
	Type                    string
	Permission              string
	PrerequisitePermission  string
	KeepCPUAwake            bool
	KeepWiFiAwake           bool
	NeedsWakeLockPermission bool
}

func backgroundServiceManifestData(androidMeta *metadata.Android) (*backgroundServiceTmplData, error) {
	if androidMeta == nil || androidMeta.BackgroundService == nil {
		return nil, nil
	}

	config := androidMeta.BackgroundService
	data := &backgroundServiceTmplData{
		Type:                    string(config.Type),
		KeepCPUAwake:            config.KeepCPUAwake,
		KeepWiFiAwake:           config.KeepWiFiAwake,
		NeedsWakeLockPermission: config.KeepCPUAwake || config.KeepWiFiAwake,
	}

	switch config.Type {
	case metadata.AndroidBackgroundServiceMediaPlayback:
		data.Permission = "android.permission.FOREGROUND_SERVICE_MEDIA_PLAYBACK"
	case metadata.AndroidBackgroundServiceConnectedDevice:
		data.Permission = "android.permission.FOREGROUND_SERVICE_CONNECTED_DEVICE"
		// Android requires a connected-device service to hold at least one
		// transport permission. CHANGE_NETWORK_STATE is a normal permission and
		// avoids imposing a runtime-granted Bluetooth or UWB permission.
		data.PrerequisitePermission = "android.permission.CHANGE_NETWORK_STATE"
	default:
		return nil, fmt.Errorf("unsupported Android.BackgroundService.Type %q", config.Type)
	}

	return data, nil
}

func requiresAAPT2(adaptiveIcon bool, backgroundService *backgroundServiceTmplData) bool {
	return adaptiveIcon || backgroundService != nil
}

func goAndroidBuild(pkg *packages.Package, bundleID string, androidArchs []string,
	iconPath, appName, version string, build, target int, distribution bool, iconFG, iconBG, iconMono string,
	androidMeta *metadata.Android,
) (map[string]bool, error) {
	// Every Android build needs 16 KB ELF alignment, not just the ones headed for
	// the Play Store: a 4 KB aligned library makes Android 15+ devices with 16 KB
	// pages run the app in page size compat mode and warn the user about it. The
	// NDK only started defaulting to it in r28. The value carries no spaces, so
	// the quoting the go tool applies to CGO_LDFLAGS leaves it alone.
	ldflags := "-Wl,-z,max-page-size=16384"
	if current := os.Getenv("CGO_LDFLAGS"); current != "" {
		ldflags = current + " " + ldflags
	}
	env := []string{"CGO_LDFLAGS=" + ldflags}

	ndkRoot, err := ndkRoot()
	if err != nil {
		return nil, err
	}
	libName := androidPkgName(appName)

	// TODO(hajimehoshi): This works only with Go tools that assume all source files are in one directory.
	// Fix this to work with other Go tools.
	dir := filepath.Dir(pkg.GoFiles[0])

	backgroundService, err := backgroundServiceManifestData(androidMeta)
	if err != nil {
		return nil, err
	}
	foreground, _, _ := detectAdaptiveIcons(dir, iconFG, iconBG, iconMono)
	adaptive := foreground != "" && util.Exists(foreground)
	useAAPT2 := requiresAAPT2(adaptive, backgroundService)

	manifestPath := filepath.Join(dir, "AndroidManifest.xml")
	manifestData, err := os.ReadFile(filepath.Clean(manifestPath))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		tmplData := manifestTmplData{
			JavaPkgPath: bundleID,
			Name:        strings.Title(appName), //lint:ignore SA1019 It is fine for our uses.
			// -release is what asks for debug support to be stripped out. Tying
			// this to -distribution instead would make every APK debuggable,
			// since that flag emits an .aab.
			Debug:             !buildRelease && !buildDistribution,
			LibName:           libName,
			Version:           version,
			Build:             build,
			AdaptiveIcon:      adaptive,
			LegacyIcon:        useAAPT2 && !adaptive && resolveLegacyIconPath(dir, iconPath) != "",
			BackgroundService: backgroundService,
		}
		if androidMeta != nil {
			tmplData.ShareMimeTypes = androidMeta.ShareMimeTypes
			tmplData.MulticastDiscovery = androidMeta.MulticastDiscovery
			tmplData.BatteryOptimizationExemption = androidMeta.BatteryOptimizationExemption
		}

		buf := new(bytes.Buffer)
		buf.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
		err := templates.ManifestAndroid.Execute(buf, tmplData)
		if err != nil {
			return nil, err
		}
		manifestData = buf.Bytes()
		if buildV {
			fmt.Fprintf(os.Stderr, "generated AndroidManifest.xml:\n%s\n", manifestData)
		}
	} else {
		libName, err = manifestLibName(manifestData)
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %v", manifestPath, err)
		}
	}

	libFiles := []string{}
	nmpkgs := make(map[string]map[string]bool) // map: arch -> extractPkgs' output

	for _, arch := range androidArchs {
		toolchain := ndk.Toolchain(arch)
		libPath := "lib/" + toolchain.abi + "/lib" + libName + ".so"
		libAbsPath := filepath.Join(tmpdir, libPath)
		if err := mkdir(filepath.Dir(libAbsPath)); err != nil {
			return nil, err
		}
		// If building release and no ldflags are set then remove the useless debug and DWARF build options
		if distribution && buildLdflags == "" {
			buildLdflags = "-w" // gomobile requires symbol check, so "-s" cannot be used yet - TODO resolve this
		}
		err = goBuild(
			pkg.PkgPath,
			append(env, androidEnv[arch]...),
			"-buildmode=c-shared",
			"-o", libAbsPath,
		)
		if err != nil {
			return nil, err
		}
		nmpkgs[arch], err = extractPkgs(toolchain.Path(ndkRoot, "nm"), libAbsPath)
		if err != nil {
			return nil, err
		}
		libFiles = append(libFiles, libPath)
	}

	ext := ".apk"
	if distribution {
		ext = ".aab"
	}
	if buildO == "" {
		buildO = androidPkgName(appName) + ext
	}
	if !strings.HasSuffix(buildO, ext) {
		return nil, fmt.Errorf("output file name %q does not end in '%s", buildO, ext)
	}

	var out io.Writer
	if !buildN {
		f, err := os.Create(buildO[:len(buildO)-3] + "apk")
		if err != nil {
			return nil, err
		}
		defer func() {
			if cerr := f.Close(); err == nil {
				err = cerr
			}
		}()
		out = f
	}

	apkw, err := buildAPK(out, nmpkgs, libFiles, androidArchs)
	if err != nil {
		return nil, err
	}
	err = addAssets(apkw, manifestData, dir, iconPath, target, build, version, iconFG, iconBG, iconMono, useAAPT2)
	if err != nil {
		return nil, err
	}

	// TODO: add gdbserver to apk?

	if !buildN {
		if err := apkw.Close(); err != nil {
			return nil, err
		}
	}
	if distribution {
		_, err := exec.LookPath("bundletool")
		if err != nil {
			_, _ = fmt.Fprint(os.Stderr, "Required command 'bundletool' not found when building Android for release.\n")
			_, _ = fmt.Fprint(os.Stderr, "For more information see https://developer.android.com/tools/bundletool.\n")
			return nil, fmt.Errorf("bundletool: command not found")
		}
		err = convertAPKToAAB(buildO)
		if err != nil {
			return nil, err
		}
	}

	// TODO: return nmpkgs
	return nmpkgs[androidArchs[0]], nil
}

// detectAdaptiveIcons checks for adaptive icon layers based on metadata or convention
// Returns: foreground path, background path, monochrome path (empty strings if not found)
func detectAdaptiveIcons(dir, foreground, background, monochrome string) (string, string, string) {
	if !util.Exists(foreground) {
		foreground = filepath.Join(dir, "Icon-foreground.png")
	}
	if !util.Exists(background) {
		background = filepath.Join(dir, "Icon-background.png")
	}
	if !util.Exists(monochrome) {
		monochrome = filepath.Join(dir, "Icon-monochrome.png")
	}

	if !util.Exists(foreground) {
		return "", "", ""
	}

	fg := foreground
	bg := ""
	if util.Exists(background) {
		bg = background
	}
	mono := ""
	if util.Exists(monochrome) {
		mono = monochrome
	}
	return fg, bg, mono
}

func resolveLegacyIconPath(dir, iconPath string) string {
	assetIcon := filepath.Join(dir, "assets", "icon.png")
	if util.Exists(assetIcon) {
		return assetIcon
	}
	return iconPath
}

func addAssets(apkw *Writer, manifestData []byte, dir, iconPath string, target int, versionCode int,
	versionName, iconFG, iconBG, iconMono string, forceAAPT2 bool,
) error {
	// Add any assets.
	legacyIconPath := resolveLegacyIconPath(dir, iconPath)
	assetsDir := filepath.Join(dir, "assets")
	assetsDirExists := true
	fi, err := os.Stat(assetsDir)
	if err != nil {
		if os.IsNotExist(err) {
			assetsDirExists = false
		} else {
			return err
		}
	} else {
		assetsDirExists = fi.IsDir()
	}
	if assetsDirExists {
		// if assets is a symlink, follow the symlink.
		assetsDir, err = filepath.EvalSymlinks(assetsDir)
		if err != nil {
			return err
		}
		err = filepath.WalkDir(assetsDir, func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if name := filepath.Base(path); strings.HasPrefix(name, ".") {
				// Do not include the hidden files.
				return nil
			}
			if info.IsDir() {
				return nil
			}

			if rel, err := filepath.Rel(assetsDir, path); rel == "icon.png" && err == nil {
				// TODO returning here does not write the assets/icon.png to the final assets output,
				// making it unavailable via the assets API. Should the file be duplicated into assets
				// or should assets API be able to retrieve files from the generated resource table?
				return nil
			}

			name := "assets/" + path[len(assetsDir)+1:]
			return apkwWriteFile(name, path, apkw)
		})
		if err != nil {
			return fmt.Errorf("asset %v", err)
		}
	}

	iconForeground, iconBackground, iconMonochrome := detectAdaptiveIcons(dir, iconFG, iconBG, iconMono)

	if iconForeground == "" && !forceAAPT2 {
		return legacyAddAssets(apkw, manifestData, legacyIconPath, target)
	}

	if iconForeground != "" && iconBackground == "" {
		iconBackground = iconForeground
	}

	arscPath, resDir, compiledManifestPath, err := compileAndroidResources(
		tmpdir,
		manifestData,
		legacyIconPath,
		iconForeground,
		iconBackground,
		iconMonochrome,
		target,
		versionCode,
		versionName,
	)
	if err != nil {
		return fmt.Errorf("failed to compile Android resources: %w", err)
	}

	if arscPath != "" {
		w, err := apkwCreate("resources.arsc", apkw)
		if err != nil {
			return err
		}
		arscData, err := os.ReadFile(arscPath)
		if err != nil {
			return err
		}
		if _, err := w.Write(arscData); err != nil {
			return err
		}
	}

	err = filepath.Walk(resDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(resDir, path)
		if err != nil {
			return err
		}
		return apkwWriteFile("res/"+relPath, path, apkw)
	})
	if err != nil {
		return fmt.Errorf("failed to write res directory: %w", err)
	}

	return apkwWriteFile("AndroidManifest.xml", compiledManifestPath, apkw)
}

func legacyAddAssets(apkw *Writer, manifestData []byte, iconPath string, target int) error {
	// Legacy single icon mode using binres
	bxml, err := binres.UnmarshalXML(bytes.NewReader(manifestData), iconPath != "", target)
	if err != nil {
		return err
	}

	// generate resources.arsc identifying single xxxhdpi icon resource.
	if iconPath != "" {
		pkgname, err := bxml.RawValueByName("manifest", xml.Name{Local: "package"})
		if err != nil {
			return err
		}
		tbl, name := binres.NewMipmapTable(pkgname)
		if err := apkwWriteFile(name, iconPath, apkw); err != nil {
			return err
		}
		w, err := apkwCreate("resources.arsc", apkw)
		if err != nil {
			return err
		}
		bin, err := tbl.MarshalBinary()
		if err != nil {
			return err
		}
		if _, err := w.Write(bin); err != nil {
			return err
		}
	}

	w, err := apkwCreate("AndroidManifest.xml", apkw)
	if err != nil {
		return err
	}
	bin, err := bxml.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err := w.Write(bin); err != nil {
		return err
	}
	return nil
}

func buildAPK(out io.Writer, nmpkgs map[string]map[string]bool, libFiles []string, androidArchs []string) (*Writer, error) {
	block, _ := pem.Decode([]byte(debugCert))
	if block == nil {
		return nil, errors.New("no debug cert")
	}
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	var apkw *Writer
	if !buildN {
		apkw = NewWriter(out, privKey)
	}

	w, err := apkwCreate("classes.dex", apkw)
	if err != nil {
		return nil, err
	}
	dexData, err := base64.StdEncoding.DecodeString(dexStr)
	if err != nil {
		log.Fatalf("internal error bad dexStr: %v", err)
	}
	if _, err := w.Write(dexData); err != nil {
		return nil, err
	}

	for _, libFile := range libFiles {
		if err := apkwWriteFile(libFile, filepath.Join(tmpdir, libFile), apkw); err != nil {
			return nil, err
		}
	}

	for _, arch := range androidArchs {
		toolchain := ndk.Toolchain(arch)
		if nmpkgs[arch]["github.com/alexballas/refyne/v2/internal/driver/mobile/exp/audio/al"] {
			dst := "lib/" + toolchain.abi + "/libopenal.so"
			src := filepath.Join(gomobilepath, dst)
			if _, err := os.Stat(src); err != nil {
				return nil, errors.New("the Android requires the github.com/alexballas/refyne/v2/internal/driver/mobile/exp/audio/al, but the OpenAL libraries was not found. Please run gomobile init with the -openal Flag pointing to an OpenAL source directory")
			}
			if err := apkwWriteFile(dst, src, apkw); err != nil {
				return nil, err
			}
		}
	}
	return apkw, nil
}

func apkwCreate(name string, apkw *Writer) (io.Writer, error) {
	if buildV {
		fmt.Fprintf(os.Stderr, "apk: %s\n", name)
	}
	if buildN {
		return io.Discard, nil
	}
	return apkw.Create(name)
}

func apkwWriteFile(dst, src string, apkw *Writer) error {
	w, err := apkwCreate(dst, apkw)
	if err != nil {
		return err
	}
	if !buildN {
		f, err := os.Open(filepath.Clean(src))
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := io.Copy(w, f); err != nil {
			return err
		}
	}
	return nil
}

// androidPkgName sanitizes the go package name to be acceptable as a android
// package name part. The android package name convention is similar to the
// java package name convention described in
// https://docs.oracle.com/javase/specs/jls/se8/html/jls-6.html#jls-6.5.3.1
// but not exactly same.
func androidPkgName(name string) string {
	var res []rune
	for _, r := range name {
		switch {
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
			res = append(res, r)
		default:
			res = append(res, '_')
		}
	}
	if len(res) == 0 || res[0] == '_' || ('0' <= res[0] && res[0] <= '9') {
		// Android does not seem to allow the package part starting with _.
		res = append([]rune{'g', 'o'}, res...)
	}
	s := string(res)
	// Look for Java keywords that are not Go keywords, and avoid using
	// them as a package name.
	//
	// This is not a problem for normal Go identifiers as we only expose
	// exported symbols. The upper case first letter saves everything
	// from accidentally matching except for the package name.
	//
	// Note that basic type names (like int) are not keywords in Go.
	switch s {
	case "abstract", "assert", "boolean", "byte", "catch", "char", "class",
		"do", "double", "enum", "extends", "final", "finally", "float",
		"implements", "instanceof", "int", "long", "native", "private",
		"protected", "public", "short", "static", "strictfp", "super",
		"synchronized", "this", "throw", "throws", "transient", "try",
		"void", "volatile", "while":
		s += "_"
	}
	return s
}

func convertAPKToAAB(aabPath string) error {
	apkPath := buildO[:len(aabPath)-3] + "apk"
	apkProtoPath := buildO[:len(aabPath)-3] + "apk-proto"
	tmpPath := filepath.Join(filepath.Dir(aabPath), "tmpbundle")
	err := os.MkdirAll(tmpPath, 0o755)
	if err != nil {
		return err
	}
	defer removeAll(tmpPath)

	aapt2, err := util.Aapt2Path()
	if err != nil {
		return err
	}
	cmd := exec.Command(aapt2, "convert", "--output-format", "proto", "-o", apkProtoPath, apkPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return err
	}
	_ = os.Remove(apkPath)

	cmd = exec.Command("unzip", apkProtoPath, "-x", "META-INF/*", "-d", tmpPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return err
	}
	_ = os.Remove(apkProtoPath)

	_ = os.MkdirAll(filepath.Join(tmpPath, "dex"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmpPath, "manifest"), 0o755)
	_ = os.Rename(filepath.Join(tmpPath, "AndroidManifest.xml"), filepath.Join(tmpPath, "manifest", "AndroidManifest.xml"))
	_ = os.Rename(filepath.Join(tmpPath, "classes.dex"), filepath.Join(tmpPath, "dex", "classes.dex"))

	cmd = exec.Command("zip", "../base.zip", "-r", ".")
	cmd.Dir = tmpPath
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return err
	}
	defer os.Remove(filepath.Join(filepath.Dir(aabPath), "base.zip"))

	cmd = exec.Command("bundletool", "build-bundle", "--output", aabPath, "--modules", "base.zip")
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// A random uninteresting private key.
// Must be consistent across builds so newer app versions can be installed.
const debugCert = `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAy6ItnWZJ8DpX9R5FdWbS9Kr1U8Z7mKgqNByGU7No99JUnmyu
NQ6Uy6Nj0Gz3o3c0BXESECblOC13WdzjsH1Pi7/L9QV8jXOXX8cvkG5SJAyj6hcO
LOapjDiN89NXjXtyv206JWYvRtpexyVrmHJgRAw3fiFI+m4g4Qop1CxcIF/EgYh7
rYrqh4wbCM1OGaCleQWaOCXxZGm+J5YNKQcWpjZRrDrb35IZmlT0bK46CXUKvCqK
x7YXHgfhC8ZsXCtsScKJVHs7gEsNxz7A0XoibFw6DoxtjKzUCktnT0w3wxdY7OTj
9AR8mobFlM9W3yirX8TtwekWhDNTYEu8dwwykwIDAQABAoIBAA2hjpIhvcNR9H9Z
BmdEecydAQ0ZlT5zy1dvrWI++UDVmIp+Ve8BSd6T0mOqV61elmHi3sWsBN4M1Rdz
3N38lW2SajG9q0fAvBpSOBHgAKmfGv3Ziz5gNmtHgeEXfZ3f7J95zVGhlHqWtY95
JsmuplkHxFMyITN6WcMWrhQg4A3enKLhJLlaGLJf9PeBrvVxHR1/txrfENd2iJBH
FmxVGILL09fIIktJvoScbzVOneeWXj5vJGzWVhB17DHBbANGvVPdD5f+k/s5aooh
hWAy/yLKocr294C4J+gkO5h2zjjjSGcmVHfrhlXQoEPX+iW1TGoF8BMtl4Llc+jw
lKWKfpECgYEA9C428Z6CvAn+KJ2yhbAtuRo41kkOVoiQPtlPeRYs91Pq4+NBlfKO
2nWLkyavVrLx4YQeCeaEU2Xoieo9msfLZGTVxgRlztylOUR+zz2FzDBYGicuUD3s
EqC0Wv7tiX6dumpWyOcVVLmR9aKlOUzA9xemzIsWUwL3PpyONhKSq7kCgYEA1X2F
f2jKjoOVzglhtuX4/SP9GxS4gRf9rOQ1Q8DzZhyH2LZ6Dnb1uEQvGhiqJTU8CXxb
7odI0fgyNXq425Nlxc1Tu0G38TtJhwrx7HWHuFcbI/QpRtDYLWil8Zr7Q3BT9rdh
moo4m937hLMvqOG9pyIbyjOEPK2WBCtKW5yabqsCgYEAu9DkUBr1Qf+Jr+IEU9I8
iRkDSMeusJ6gHMd32pJVCfRRQvIlG1oTyTMKpafmzBAd/rFpjYHynFdRcutqcShm
aJUq3QG68U9EAvWNeIhA5tr0mUEz3WKTt4xGzYsyWES8u4tZr3QXMzD9dOuinJ1N
+4EEumXtSPKKDG3M8Qh+KnkCgYBUEVSTYmF5EynXc2xOCGsuy5AsrNEmzJqxDUBI
SN/P0uZPmTOhJIkIIZlmrlW5xye4GIde+1jajeC/nG7U0EsgRAV31J4pWQ5QJigz
0+g419wxIUFryGuIHhBSfpP472+w1G+T2mAGSLh1fdYDq7jx6oWE7xpghn5vb9id
EKLjdwKBgBtz9mzbzutIfAW0Y8F23T60nKvQ0gibE92rnUbjPnw8HjL3AZLU05N+
cSL5bhq0N5XHK77sscxW9vXjG0LJMXmFZPp9F6aV6ejkMIXyJ/Yz/EqeaJFwilTq
Mc6xR47qkdzu0dQ1aPm4XD7AWDtIvPo/GG2DKOucLBbQc2cOWtKS
-----END RSA PRIVATE KEY-----
`
