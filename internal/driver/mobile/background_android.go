//go:build android

package mobile

/*
#include <stdbool.h>
#include <stdlib.h>

void startBackgroundSession(uintptr_t jni_env, uintptr_t ctx, char *title, char *text,
	void *icon, int iconLen);
void stopBackgroundSession(uintptr_t jni_env, uintptr_t ctx);
void acquireMulticastLock(uintptr_t jni_env, uintptr_t ctx);
void releaseMulticastLock(uintptr_t jni_env, uintptr_t ctx);
bool isIgnoringBatteryOptimizations(uintptr_t jni_env, uintptr_t ctx);
void requestIgnoreBatteryOptimizations(uintptr_t jni_env, uintptr_t ctx);
void requestNotificationPermission(uintptr_t jni_env, uintptr_t ctx);
*/
import "C"

import (
	"unsafe"

	fyne "github.com/alexballas/refyne/v2"
	driverDefs "github.com/alexballas/refyne/v2/driver"
	"github.com/alexballas/refyne/v2/driver/mobile"
)

// Declare conformity with the optional mobile driver interfaces.
var (
	_ mobile.BackgroundSession   = (*driver)(nil)
	_ mobile.MulticastLocker     = (*driver)(nil)
	_ mobile.BatteryOptimization = (*driver)(nil)
)

// StartBackgroundSession implements driver/mobile.BackgroundSession.
func (d *driver) StartBackgroundSession(title, text string, icon fyne.Resource) {
	titleStr := C.CString(title)
	defer C.free(unsafe.Pointer(titleStr))
	textStr := C.CString(text)
	defer C.free(unsafe.Pointer(textStr))

	// The bytes are copied into a Java array before the call returns, so this
	// buffer only has to outlive the call itself.
	var (
		iconPtr unsafe.Pointer
		iconLen C.int
	)
	if icon != nil {
		if content := icon.Content(); len(content) > 0 {
			iconPtr = C.CBytes(content)
			defer C.free(iconPtr)
			iconLen = C.int(len(content))
		}
	}

	runOnAndroidContext(func(ac *driverDefs.AndroidContext) {
		C.startBackgroundSession(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), titleStr, textStr,
			iconPtr, iconLen)
	})
}

// StopBackgroundSession implements driver/mobile.BackgroundSession.
func (d *driver) StopBackgroundSession() {
	runOnAndroidContext(func(ac *driverDefs.AndroidContext) {
		C.stopBackgroundSession(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
	})
}

// RequestNotificationPermission implements driver/mobile.BackgroundSession.
func (d *driver) RequestNotificationPermission() {
	runOnAndroidContext(func(ac *driverDefs.AndroidContext) {
		C.requestNotificationPermission(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
	})
}

// AcquireMulticastLock implements driver/mobile.MulticastLocker.
func (d *driver) AcquireMulticastLock() {
	runOnAndroidContext(func(ac *driverDefs.AndroidContext) {
		C.acquireMulticastLock(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
	})
}

// ReleaseMulticastLock implements driver/mobile.MulticastLocker.
func (d *driver) ReleaseMulticastLock() {
	runOnAndroidContext(func(ac *driverDefs.AndroidContext) {
		C.releaseMulticastLock(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
	})
}

// IsIgnoringBatteryOptimizations implements driver/mobile.BatteryOptimization.
func (d *driver) IsIgnoringBatteryOptimizations() bool {
	// Assume exempt: a caller that cannot reach the platform should not go on to
	// prompt the user about something it has no way to change.
	exempt := true
	runOnAndroidContext(func(ac *driverDefs.AndroidContext) {
		exempt = bool(C.isIgnoringBatteryOptimizations(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx)))
	})
	return exempt
}

// RequestBatteryOptimizationExemption implements driver/mobile.BatteryOptimization.
func (d *driver) RequestBatteryOptimizationExemption() {
	runOnAndroidContext(func(ac *driverDefs.AndroidContext) {
		C.requestIgnoreBatteryOptimizations(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
	})
}

// runOnAndroidContext hands fn a JNI environment attached to the running
// activity. The context type is asserted rather than forced, so a call that
// arrives before the activity exists is dropped instead of panicking.
func runOnAndroidContext(fn func(*driverDefs.AndroidContext)) {
	driverDefs.RunNative(func(ctx any) error {
		if ac, ok := ctx.(*driverDefs.AndroidContext); ok {
			fn(ac)
		}
		return nil
	})
}
