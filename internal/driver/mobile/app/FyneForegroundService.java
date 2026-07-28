package org.golang.app;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.content.pm.ServiceInfo;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.drawable.Icon;
import android.net.wifi.WifiManager;
import android.os.Build;
import android.os.Bundle;
import android.os.IBinder;
import android.os.PowerManager;
import android.util.Log;

// FyneForegroundService keeps the app process out of Android's cached bucket
// while it is doing work the user expects to continue with the screen off. A
// cached process is frozen by the
// cached-app freezer within seconds of losing the foreground, which stops every
// thread: sockets go unanswered until the peer drops them, and timers that would
// renew a session never fire. A foreground service is the only thing that keeps
// the process running; battery-optimization exemptions do not, because the
// freezer keys off process state rather than the app's power allow-list.
//
// It must be declared in the application's AndroidManifest.xml, which the Fyne
// packaging tool does when FyneApp.toml configures Android.BackgroundService:
//
//   <service android:name="org.golang.app.FyneForegroundService"
//            android:foregroundServiceType="..."
//            android:exported="false" />
//
// CPU and Wi-Fi locks are independent opt-ins in that configuration. They
// address power-management behavior rather than process lifetime and should be
// requested only by apps that need them.
public class FyneForegroundService extends Service {
    static final String ACTION_START = "org.golang.app.FOREGROUND_START";
    static final String EXTRA_TITLE = "title";
    static final String EXTRA_TEXT = "text";

    private static final String CHANNEL_ID = "fyne-background";
    private static final String META_KEEP_CPU_AWAKE = "org.fyne.background.KEEP_CPU_AWAKE";
    private static final String META_KEEP_WIFI_AWAKE = "org.fyne.background.KEEP_WIFI_AWAKE";
    private static final int NOTIFICATION_ID = 0x60F0;
    private static final int UNKNOWN_APP_ICON = 17629184; // android.R.drawable.sym_def_app_icon

    private static final ForegroundServiceLifecycle lifecycle = new ForegroundServiceLifecycle();

    private int foregroundServiceType;
    private boolean keepCPUAwake;
    private boolean keepWiFiAwake;
    private PowerManager.WakeLock wakeLock;
    private WifiManager.WifiLock wifiLock;

    // The status bar stencil the app supplied, shared with the activity that
    // starts us; both run in this process.
    private static Bitmap smallIcon;

    // setSmallIcon takes the encoded image the app passed to StartBackgroundSession.
    // A null or undecodable one leaves the launcher icon in charge.
    static void setSmallIcon(byte[] encoded) {
        if (encoded == null || encoded.length == 0) {
            smallIcon = null;
            return;
        }
        try {
            smallIcon = BitmapFactory.decodeByteArray(encoded, 0, encoded.length);
        } catch (RuntimeException e) {
            Log.e("Fyne", "could not decode the background session icon", e);
            smallIcon = null;
        }
    }

    static long startRequested() {
        return lifecycle.startRequested();
    }

    static void startFailed(long startGeneration) {
        lifecycle.startFailed(startGeneration);
    }

    static boolean deferStopIfStarting() {
        return lifecycle.deferStopIfStarting();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    @Override
    public void onCreate() {
        super.onCreate();
        loadManifestConfiguration();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        // A null intent means the system restarted us after the process died. The
        // Go state that the session belonged to is gone, so there is nothing left
        // to keep alive - START_NOT_STICKY makes this rare, but not impossible.
        if (intent == null) {
            stopSession();
            return START_NOT_STICKY;
        }

        String title = intent.getStringExtra(EXTRA_TITLE);
        String text = intent.getStringExtra(EXTRA_TEXT);

        try {
            Notification notif = buildNotification(title, text);
            // The type has to match what the manifest declares, or the platform
            // rejects the call: passing one the manifest does not list throws,
            // and so does passing none from target SDK 34 onwards. Read the
            // packaging configuration back rather than duplicating it here.
            if (foregroundServiceType != 0) {
                startForeground(NOTIFICATION_ID, notif, foregroundServiceType);
            } else {
                startForeground(NOTIFICATION_ID, notif);
            }
        } catch (RuntimeException e) {
            // Android 12+ refuses a foreground service started from the
            // background, and 14+ refuses one whose type is not declared. Neither
            // is worth taking the app down for: the caller keeps working, it just
            // loses the guarantee that it survives being backgrounded.
            Log.e("Fyne", "could not enter the foreground", e);
            lifecycle.stopped();
            stopSession();
            return START_NOT_STICKY;
        }

        // A stop can race the asynchronous delivery of onStartCommand(). Calling
        // Context.stopService() during that window makes Android crash the app
        // because this service has not entered the foreground yet. Satisfy the
        // contract above, then honor the deferred stop from inside the service.
        if (lifecycle.foregroundStarted()) {
            stopSession();
            return START_NOT_STICKY;
        }

        acquireLocks();
        return START_NOT_STICKY;
    }

    @Override
    public void onDestroy() {
        lifecycle.stopped();
        releaseLocks();
        super.onDestroy();
    }

    private void stopSession() {
        releaseLocks();
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(Service.STOP_FOREGROUND_REMOVE);
        } else {
            stopForeground(true);
        }
        stopSelf();
    }

    private void loadManifestConfiguration() {
        try {
            ServiceInfo info = getPackageManager().getServiceInfo(
                    new ComponentName(this, FyneForegroundService.class), PackageManager.GET_META_DATA);
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                foregroundServiceType = info.getForegroundServiceType();
            }
            Bundle metadata = info.metaData;
            if (metadata != null) {
                keepCPUAwake = metadata.getBoolean(META_KEEP_CPU_AWAKE, false);
                keepWiFiAwake = metadata.getBoolean(META_KEEP_WIFI_AWAKE, false);
            }
        } catch (PackageManager.NameNotFoundException e) {
            Log.e("Fyne", "could not read the foreground service configuration", e);
        }
    }

    private Notification buildNotification(String title, String text) {
        Notification.Builder builder;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationManager mgr = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            if (mgr != null) {
                // IMPORTANCE_LOW: this is an ongoing status entry, so it must not
                // make a sound or interrupt every time a session starts.
                NotificationChannel channel = new NotificationChannel(CHANNEL_ID,
                        "Background activity", NotificationManager.IMPORTANCE_LOW);
                channel.setShowBadge(false);
                mgr.createNotificationChannel(channel);
            }
            builder = new Notification.Builder(this, CHANNEL_ID);
        } else {
            builder = new Notification.Builder(this);
        }

        builder.setContentTitle(title != null ? title : "");
        builder.setContentText(text != null ? text : "");
        applySmallIcon(builder);
        builder.setOngoing(true);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q
                    && (foregroundServiceType & ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK) != 0) {
                builder.setCategory(Notification.CATEGORY_TRANSPORT);
            } else {
                builder.setCategory(Notification.CATEGORY_SERVICE);
            }
            builder.setVisibility(Notification.VISIBILITY_PUBLIC);
        }

        PendingIntent content = launchIntent();
        if (content != null) {
            builder.setContentIntent(content);
        }
        return builder.build();
    }

    // applySmallIcon prefers the stencil the app supplied. Falling back to the
    // launcher icon is not pretty - Android keeps only its alpha channel, and a
    // full-bleed launcher icon is opaque to the edges, so it lands as a white
    // block - but it is still the app's own shape rather than the stand-in
    // "unknown app" android, which is what the platform gives us otherwise.
    private void applySmallIcon(Notification.Builder builder) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M && smallIcon != null) {
            builder.setSmallIcon(Icon.createWithBitmap(smallIcon));
            return;
        }

        int res = getApplicationInfo().icon;
        builder.setSmallIcon(res != 0 ? res : UNKNOWN_APP_ICON);
    }

    // launchIntent returns to the running app when the notification is tapped. It
    // is the launcher's own MAIN intent rather than a fresh Intent for the
    // activity class, so an app that also handles shares does not see this as a
    // new ACTION_SEND / ACTION_VIEW delivery.
    private PendingIntent launchIntent() {
        Intent intent = getPackageManager().getLaunchIntentForPackage(getPackageName());
        if (intent == null) {
            return null;
        }

        int flags = PendingIntent.FLAG_UPDATE_CURRENT;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            flags |= PendingIntent.FLAG_IMMUTABLE;
        }
        return PendingIntent.getActivity(this, 0, intent, flags);
    }

    private void acquireLocks() {
        if (keepCPUAwake && wakeLock == null) {
            PowerManager power = (PowerManager) getSystemService(Context.POWER_SERVICE);
            if (power != null) {
                wakeLock = power.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "fyne:background");
                wakeLock.setReferenceCounted(false);
            }
        }
        if (wakeLock != null && !wakeLock.isHeld()) {
            wakeLock.acquire();
        }

        if (keepWiFiAwake && wifiLock == null) {
            // The application context outlives this service; a WifiManager from a
            // shorter-lived context is a documented leak.
            WifiManager wifi = (WifiManager) getApplicationContext().getSystemService(Context.WIFI_SERVICE);
            if (wifi != null) {
                // LOW_LATENCY is active only while the screen and app are in the
                // foreground. HIGH_PERF is the mode intended to keep an existing
                // connection responsive while the screen is off; newer Android
                // releases may substitute their preferred implementation.
                wifiLock = wifi.createWifiLock(WifiManager.WIFI_MODE_FULL_HIGH_PERF,
                        "fyne:background");
                wifiLock.setReferenceCounted(false);
            }
        }
        if (wifiLock != null && !wifiLock.isHeld()) {
            wifiLock.acquire();
        }
    }

    private void releaseLocks() {
        if (wifiLock != null && wifiLock.isHeld()) {
            wifiLock.release();
        }
        if (wakeLock != null && wakeLock.isHeld()) {
            wakeLock.release();
        }
    }
}
