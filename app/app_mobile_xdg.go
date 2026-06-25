//go:build !ci && mobile && !android && !ios

package app

import (
	"errors"
	"net/url"
	"time"

	fyne "github.com/alexballas/refyne/v2"
)

func (a *fyneApp) OpenURL(_ *url.URL) error {
	return errors.New("mobile simulator does not support open URLs yet")
}

func (a *fyneApp) SendNotification(_ *fyne.Notification) {
	fyne.LogError("Notifications are not supported in the mobile simulator yet", nil)
}

func (a *fyneApp) ScheduleNotification(n *fyne.Notification, when time.Time) (*fyne.ScheduledNotification, error) {
	return a.scheduleViaScheduler(n, when)
}

func (a *fyneApp) CancelScheduledNotification(id string) error {
	return a.cancelViaScheduler(id)
}

func watchTheme(_ *settings) {
	// not implemented yet
}
