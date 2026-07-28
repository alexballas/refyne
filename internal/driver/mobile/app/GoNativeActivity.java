package org.golang.app;

import android.app.Activity;
import android.app.AlarmManager;
import android.app.NativeActivity;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ActivityInfo;
import android.content.pm.PackageManager;
import android.content.res.Configuration;
import android.graphics.Rect;
import android.net.Uri;
import android.net.wifi.WifiManager;
import android.os.Build;
import android.os.Bundle;
import android.os.Parcelable;
import android.os.PowerManager;
import android.provider.Settings;
import android.text.Editable;
import android.text.InputType;
import android.text.TextWatcher;
import android.text.method.DigitsKeyListener;
import android.util.Log;
import android.view.Gravity;
import android.view.KeyCharacterMap;
import android.view.View;
import android.view.ViewGroup;
import android.view.WindowInsets;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputMethodManager;
import android.view.KeyEvent;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.TextView;
import android.widget.TextView.OnEditorActionListener;

import java.util.ArrayList;
import java.util.List;

public class GoNativeActivity extends NativeActivity {
	private static GoNativeActivity goNativeActivity;
	private static final int FILE_OPEN_CODE = 1;
	private static final int FILE_SAVE_CODE = 2;
	private static final int NOTIFICATION_PERMISSION_CODE = 3;

	private static final int DEFAULT_INPUT_TYPE = InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS;

	private static final int DEFAULT_KEYBOARD_CODE = 0;
	private static final int SINGLELINE_KEYBOARD_CODE = 1;
	private static final int NUMBER_KEYBOARD_CODE = 2;
	private static final int PASSWORD_KEYBOARD_CODE = 3;

    private native void filePickerReturned(String str);
    private native void intentReceived(String uri, String mime);
    private native void insetsChanged(int top, int bottom, int left, int right);
    private native void keyboardTyped(String str);
    private native void keyboardDelete();
    private native void backPressed();
    private native void setDarkMode(boolean dark);

	private EditText mTextEdit;
	private boolean ignoreKey = false;
	private boolean keyboardUp = false;

	// Identifies the share / open-with intent most recently passed to Go. It is
	// static on purpose: Activity recreation (rotation, density change, forced
	// recreation) re-delivers the *original launch* intent rather than whatever
	// setIntent stored, so an instance field would replay the share every time.
	// The Go runtime is not restarted on recreation, so a process-scoped marker
	// matches the lifetime of the state it protects; after process death it is
	// gone and a replay is correct, because the restored UI is empty.
	private static String handledIntentKey;

	// Accessibility – real-view overlay approach.
	private static FrameLayout mA11yContainer;

	// Staging buffer – written by Go threads, consumed on the UI thread.
	private static final List<int[]>  sStagingData   = new ArrayList<>();
	private static final List<String> sStagingLabels = new ArrayList<>();
	// Signature of the last committed layout; skip UI work when nothing changed.
	private static String sLastCommittedSignature = null;

	public GoNativeActivity() {
		super();
		goNativeActivity = this;
	}

	String getTmpdir() {
		return getCacheDir().getAbsolutePath();
	}

	void updateLayout() {
	    try {
            WindowInsets insets = getWindow().getDecorView().getRootWindowInsets();
            if (insets == null) {
                return;
            }

            insetsChanged(insets.getSystemWindowInsetTop(), insets.getSystemWindowInsetBottom(),
                insets.getSystemWindowInsetLeft(), insets.getSystemWindowInsetRight());
        } catch (java.lang.NoSuchMethodError e) {
    	    Rect insets = new Rect();
            getWindow().getDecorView().getWindowVisibleDisplayFrame(insets);

            View view = findViewById(android.R.id.content).getRootView();
            insetsChanged(insets.top, view.getHeight() - insets.height() - insets.top,
                insets.left, view.getWidth() - insets.width() - insets.left);
        }
    }

    static void showKeyboard(int keyboardType) {
        goNativeActivity.doShowKeyboard(keyboardType);
        goNativeActivity.keyboardUp = true;
    }

    void doShowKeyboard(final int keyboardType) {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                int imeOptions = EditorInfo.IME_FLAG_NO_ENTER_ACTION;
                int inputType = DEFAULT_INPUT_TYPE;
                String keys = "";
                switch (keyboardType) {
                    case DEFAULT_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_FLAG_NO_ENTER_ACTION;
                        break;
                    case SINGLELINE_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_ACTION_DONE;
                        break;
                    case NUMBER_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_ACTION_DONE;
                        inputType |= InputType.TYPE_CLASS_NUMBER | InputType.TYPE_NUMBER_VARIATION_NORMAL;
                        keys = "0123456789.,-' "; // work around android bug where some number keys are blocked
                        break;
                    case PASSWORD_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_ACTION_DONE;
                        inputType |= InputType.TYPE_TEXT_VARIATION_PASSWORD;
                    default:
                        Log.e("Fyne", "unknown keyboard type, use default");
                }
                mTextEdit.setImeOptions(imeOptions|EditorInfo.IME_FLAG_NO_FULLSCREEN);
                mTextEdit.setInputType(inputType);
                if (keys != "") {
                    mTextEdit.setKeyListener(DigitsKeyListener.getInstance(keys));
                }

                mTextEdit.setOnEditorActionListener(new OnEditorActionListener() {
                    @Override
                    public boolean onEditorAction(TextView v, int actionId, KeyEvent event) {
                        if (actionId == EditorInfo.IME_ACTION_DONE) {
                            keyboardTyped("\n");
                        }
                        return false;
                    }
                });

                // always place one character so all keyboards can send backspace
                ignoreKey = true;
                mTextEdit.setText(" ");
                mTextEdit.setSelection(mTextEdit.getText().length());
                ignoreKey = false;

                mTextEdit.setVisibility(View.VISIBLE);
                mTextEdit.bringToFront();
                mTextEdit.requestFocus();

                InputMethodManager m = (InputMethodManager) getSystemService(Context.INPUT_METHOD_SERVICE);
                m.showSoftInput(mTextEdit, 0);
            }
        });
    }

    static void hideKeyboard() {
        goNativeActivity.doHideKeyboard();
        goNativeActivity.keyboardUp = false;
    }

    void doHideKeyboard() {
        InputMethodManager imm = (InputMethodManager) getSystemService(Context.INPUT_METHOD_SERVICE);
        View view = findViewById(android.R.id.content).getRootView();
        imm.hideSoftInputFromWindow(view.getWindowToken(), 0);

        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                mTextEdit.setVisibility(View.GONE);
            }
        });
    }

    static void showFileOpen(String mimes) {
        goNativeActivity.doShowFileOpen(mimes);
    }

    void doShowFileOpen(String mimes) {
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        if ("application/x-directory".equals(mimes) && Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
            intent = new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE); // ask for a directory picker if OS supports it
            intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        } else if (mimes.contains("|") && Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) {
            intent.setType("*/*");
            intent.putExtra(Intent.EXTRA_MIME_TYPES, mimes.split("\\|"));
            intent.addCategory(Intent.CATEGORY_OPENABLE);
        } else {
            intent.setType(mimes);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
        }
        startActivityForResult(Intent.createChooser(intent, "Open File"), FILE_OPEN_CODE);
    }

    static void showFileSave(String mimes, String filename) {
        goNativeActivity.doShowFileSave(mimes, filename);
    }

    void doShowFileSave(String mimes, String filename) {
        Intent intent = new Intent(Intent.ACTION_CREATE_DOCUMENT);
        if (mimes.contains("|") && Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) {
            intent.setType("*/*");
            intent.putExtra(Intent.EXTRA_MIME_TYPES, mimes.split("\\|"));
        } else {
            intent.setType(mimes);
        }
        intent.putExtra(Intent.EXTRA_TITLE, filename);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        startActivityForResult(Intent.createChooser(intent, "Save File"), FILE_SAVE_CODE);
    }

    // Scheduled notifications via AlarmManager.
    //
    // For delivery to survive the app process being killed, FyneNotificationReceiver
    // must be declared in AndroidManifest.xml:
    //
    //   <receiver android:name="org.golang.app.FyneNotificationReceiver"
    //             android:exported="false" />
    //
    // If the receiver is not registered the schedule call returns false and the
    // Go layer falls back to an in-process scheduler.
    static boolean scheduleNotification(String id, String title, String body, long deliveryTimeMillis) {
        if (goNativeActivity == null) {
            return false;
        }
        return goNativeActivity.doScheduleNotification(id, title, body, deliveryTimeMillis);
    }

    boolean doScheduleNotification(String id, String title, String body, long deliveryTimeMillis) {
        ComponentName receiver = new ComponentName(this, FyneNotificationReceiver.class);
        try {
            getPackageManager().getReceiverInfo(receiver, 0);
        } catch (PackageManager.NameNotFoundException e) {
            return false;
        }

        AlarmManager alarmMgr = (AlarmManager) getSystemService(Context.ALARM_SERVICE);
        if (alarmMgr == null) {
            return false;
        }

        Intent intent = new Intent(this, FyneNotificationReceiver.class);
        intent.putExtra("title", title);
        intent.putExtra("body", body);
        intent.putExtra("notif_id", id.hashCode());

        int flags = PendingIntent.FLAG_UPDATE_CURRENT;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            flags |= PendingIntent.FLAG_IMMUTABLE;
        }

        PendingIntent pi = PendingIntent.getBroadcast(this, id.hashCode(), intent, flags);
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
                alarmMgr.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, deliveryTimeMillis, pi);
            } else {
                alarmMgr.set(AlarmManager.RTC_WAKEUP, deliveryTimeMillis, pi);
            }
        } catch (SecurityException e) {
            Log.e("Fyne", "AlarmManager rejected scheduled notification", e);
            return false;
        }
        return true;
    }

    static void cancelScheduledNotification(String id) {
        if (goNativeActivity == null) {
            return;
        }
        goNativeActivity.doCancelScheduledNotification(id);
    }

    void doCancelScheduledNotification(String id) {
        AlarmManager alarmMgr = (AlarmManager) getSystemService(Context.ALARM_SERVICE);
        if (alarmMgr == null) {
            return;
        }

        Intent intent = new Intent(this, FyneNotificationReceiver.class);
        int flags = PendingIntent.FLAG_NO_CREATE;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            flags |= PendingIntent.FLAG_IMMUTABLE;
        }

        PendingIntent pi = PendingIntent.getBroadcast(this, id.hashCode(), intent, flags);
        if (pi != null) {
            alarmMgr.cancel(pi);
            pi.cancel();
        }

        NotificationManager mgr = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        if (mgr != null) {
            mgr.cancel(id.hashCode());
        }
    }

    // Background session, Wi-Fi multicast and battery-optimisation bridges.
    //
    // Each of these depends on a manifest entry the packaging tool only writes
    // when FyneApp.toml opts in, so every one of them has to degrade quietly when
    // the entry is absent: an app that never asked to run in the background must
    // not crash the first time a library calls one of these.

    static void startBackgroundSession(String title, String text, byte[] icon) {
        if (goNativeActivity == null) {
            return;
        }
        // Handed over before the service starts rather than as an intent extra:
        // both live in this process, so there is no reason to push the image
        // through Binder on every state change.
        FyneForegroundService.setSmallIcon(icon);
        goNativeActivity.doStartBackgroundSession(title, text);
    }

    void doStartBackgroundSession(String title, String text) {
        ComponentName service = new ComponentName(this, FyneForegroundService.class);
        try {
            getPackageManager().getServiceInfo(service, 0);
        } catch (PackageManager.NameNotFoundException e) {
            Log.e("Fyne", "FyneForegroundService is not declared in AndroidManifest.xml");
            return;
        }

        Intent intent = new Intent(this, FyneForegroundService.class);
        intent.setAction(FyneForegroundService.ACTION_START);
        intent.putExtra(FyneForegroundService.EXTRA_TITLE, title);
        intent.putExtra(FyneForegroundService.EXTRA_TEXT, text);

        long startGeneration = FyneForegroundService.startRequested();
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                startForegroundService(intent);
            } else {
                startService(intent);
            }
        } catch (RuntimeException e) {
            // Android 12+ throws when a foreground service is started from the
            // background. The caller cannot always know it lost the foreground
            // between deciding to start and getting here.
            Log.e("Fyne", "could not start the background session", e);
            FyneForegroundService.startFailed(startGeneration);
        }
    }

    static void stopBackgroundSession() {
        if (goNativeActivity == null) {
            return;
        }
        goNativeActivity.doStopBackgroundSession();
    }

    void doStopBackgroundSession() {
        // startForegroundService() and Service.startForeground() are separate
        // operations. Do not cancel the service between them: Android treats
        // that as a failed promotion and crashes the app. The service consumes
        // this deferred stop immediately after entering the foreground.
        if (FyneForegroundService.deferStopIfStarting()) {
            return;
        }

        Intent intent = new Intent(this, FyneForegroundService.class);
        try {
            stopService(intent);
        } catch (RuntimeException e) {
            Log.e("Fyne", "could not stop the background session", e);
        }
    }

    // Wi-Fi drops multicast and broadcast frames that are not addressed to the
    // device unless a multicast lock is held, which is what SSDP and mDNS
    // discovery rely on. The lock is process-scoped rather than tied to the
    // background service, because discovery also runs with the app in front.
    private static WifiManager.MulticastLock multicastLock;

    static synchronized void acquireMulticastLock() {
        if (goNativeActivity == null) {
            return;
        }
        goNativeActivity.doAcquireMulticastLock();
    }

    void doAcquireMulticastLock() {
        if (multicastLock == null) {
            WifiManager wifi = (WifiManager) getApplicationContext().getSystemService(Context.WIFI_SERVICE);
            if (wifi == null) {
                return;
            }
            multicastLock = wifi.createMulticastLock("fyne:discovery");
            // Reference counted: overlapping discovery runs are normal, and the
            // lock has to survive until the last one finishes.
            multicastLock.setReferenceCounted(true);
        }

        try {
            multicastLock.acquire();
        } catch (RuntimeException e) {
            // Missing CHANGE_WIFI_MULTICAST_STATE. Discovery still works on
            // networks that do not filter, so this is not fatal.
            Log.e("Fyne", "could not acquire the multicast lock", e);
        }
    }

    static synchronized void releaseMulticastLock() {
        if (multicastLock == null || !multicastLock.isHeld()) {
            return;
        }
        try {
            multicastLock.release();
        } catch (RuntimeException e) {
            Log.e("Fyne", "could not release the multicast lock", e);
        }
    }

    static boolean isIgnoringBatteryOptimizations() {
        if (goNativeActivity == null || Build.VERSION.SDK_INT < Build.VERSION_CODES.M) {
            // Before Marshmallow there is no doze to be exempt from.
            return true;
        }
        return goNativeActivity.doIsIgnoringBatteryOptimizations();
    }

    boolean doIsIgnoringBatteryOptimizations() {
        PowerManager power = (PowerManager) getSystemService(Context.POWER_SERVICE);
        if (power == null) {
            return true;
        }
        return power.isIgnoringBatteryOptimizations(getPackageName());
    }

    static void requestIgnoreBatteryOptimizations() {
        if (goNativeActivity == null || Build.VERSION.SDK_INT < Build.VERSION_CODES.M) {
            return;
        }
        goNativeActivity.doRequestIgnoreBatteryOptimizations();
    }

    void doRequestIgnoreBatteryOptimizations() {
        // Go calls in on a JNI thread, and starting an activity belongs on the UI
        // thread.
        runOnUiThread(new Runnable() {
            public void run() {
                // The one-tap dialog needs REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                // which Google Play restricts to a few app categories. Apps that
                // cannot declare it get the settings list instead, where the user
                // picks the app by hand.
                if (checkSelfPermission("android.permission.REQUEST_IGNORE_BATTERY_OPTIMIZATIONS")
                        == PackageManager.PERMISSION_GRANTED) {
                    Intent direct = new Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS,
                            Uri.parse("package:" + getPackageName()));
                    if (startIfResolvable(direct)) {
                        return;
                    }
                }

                startIfResolvable(new Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS));
            }
        });
    }

    private boolean startIfResolvable(Intent intent) {
        // Not every device ships the battery-optimisation screens, and an OEM that
        // renamed them would otherwise take the app down with ActivityNotFound.
        if (intent.resolveActivity(getPackageManager()) == null) {
            return false;
        }
        try {
            startActivity(intent);
            return true;
        } catch (RuntimeException e) {
            Log.e("Fyne", "could not open the battery optimisation settings", e);
            return false;
        }
    }

    // requestNotificationPermission asks for POST_NOTIFICATIONS, which Android 13
    // requires before any notification is shown - including the one that a
    // foreground service is built around. The result is not awaited: a denied
    // permission costs the notification, not the service.
    static void requestNotificationPermission() {
        if (goNativeActivity == null || Build.VERSION.SDK_INT < 33) {
            return;
        }
        goNativeActivity.doRequestNotificationPermission();
    }

    void doRequestNotificationPermission() {
        final String perm = "android.permission.POST_NOTIFICATIONS";
        if (checkSelfPermission(perm) == PackageManager.PERMISSION_GRANTED) {
            return;
        }
        runOnUiThread(new Runnable() {
            public void run() {
                try {
                    requestPermissions(new String[]{perm}, NOTIFICATION_PERMISSION_CODE);
                } catch (RuntimeException e) {
                    Log.e("Fyne", "could not request the notification permission", e);
                }
            }
        });
    }

	static int getRune(int deviceId, int keyCode, int metaState) {
		try {
			int rune = KeyCharacterMap.load(deviceId).get(keyCode, metaState);
			if (rune == 0) {
				return -1;
			}
			return rune;
		} catch (KeyCharacterMap.UnavailableException e) {
			return -1;
		} catch (Exception e) {
			Log.e("Fyne", "exception reading KeyCharacterMap", e);
			return -1;
		}
	}

	private void load() {
		// Interestingly, NativeActivity uses a different method
		// to find native code to execute, avoiding
		// System.loadLibrary. The result is Java methods
		// implemented in C with JNIEXPORT (and JNI_OnLoad) are not
		// available unless an explicit call to System.loadLibrary
		// is done. So we do it here, borrowing the name of the
		// library from the same AndroidManifest.xml metadata used
		// by NativeActivity.
		try {
			ActivityInfo ai = getPackageManager().getActivityInfo(
					getIntent().getComponent(), PackageManager.GET_META_DATA);
			if (ai.metaData == null) {
				Log.e("Fyne", "loadLibrary: no manifest metadata found");
				return;
			}
			String libName = ai.metaData.getString("android.app.lib_name");
			System.loadLibrary(libName);
		} catch (Exception e) {
			Log.e("Fyne", "loadLibrary failed", e);
		}
	}

	@Override
	public void onCreate(Bundle savedInstanceState) {
		load();
		super.onCreate(savedInstanceState);
		setupEntry();
		updateTheme(getResources().getConfiguration());

		View view = findViewById(android.R.id.content).getRootView();
		view.addOnLayoutChangeListener(new View.OnLayoutChangeListener() {
			public void onLayoutChange (View v, int left, int top, int right, int bottom,
			                            int oldLeft, int oldTop, int oldRight, int oldBottom) {
				GoNativeActivity.this.updateLayout();
			}
		});

		// load() ran System.loadLibrary above, and calling an exported Go function
		// blocks until the cgo runtime handshake completes, so the Go side can take
		// this even on a cold start. Buffering until an app handler exists happens
		// in Go, not here.
		handleIntent(getIntent());
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);
        handleIntent(intent);
    }

    // handleIntent forwards a share (ACTION_SEND) or open-with (ACTION_VIEW) intent
    // to Go. Anything else - notably the ACTION_MAIN of an ordinary launch - is
    // ignored.
    private void handleIntent(Intent intent) {
        if (intent == null) {
            return;
        }

        String action = intent.getAction();
        Uri uri;
        if (Intent.ACTION_VIEW.equals(action)) {
            uri = intent.getData();
        } else if (Intent.ACTION_SEND.equals(action)) {
            uri = getStreamExtra(intent);
        } else {
            return;
        }

        // A share carrying only EXTRA_TEXT has no stream. Returning here keeps
        // uri.toString() from throwing before Go ever sees the intent.
        if (uri == null) {
            return;
        }

        String key = action + "\n" + uri.toString();
        if (key.equals(handledIntentKey)) {
            return;
        }
        handledIntentKey = key;

        // resolveType rather than getType: an open-with URI can reach us with no
        // explicit type on the intent while the provider still knows one. It may
        // still come back null, which Go handles.
        String mime = null;
        try {
            mime = intent.resolveType(getContentResolver());
        } catch (Exception e) {
            Log.e("Fyne", "could not resolve type of incoming intent", e);
        }

        intentReceived(uri.toString(), mime);
    }

    // getStreamExtra reads EXTRA_STREAM defensively. This activity is exported, so
    // any app on the device can hand us a malformed parcel or a Parcelable that is
    // not a Uri; both mean "no URI" rather than a crash.
    private Uri getStreamExtra(Intent intent) {
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                return intent.getParcelableExtra(Intent.EXTRA_STREAM, Uri.class);
            }

            Parcelable stream = intent.getParcelableExtra(Intent.EXTRA_STREAM);
            return (stream instanceof Uri) ? (Uri) stream : null;
        } catch (Exception e) {
            Log.e("Fyne", "could not read shared stream", e);
            return null;
        }
    }

    private void setupEntry() {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                mTextEdit = new EditText(goNativeActivity);
                mTextEdit.setVisibility(View.GONE);
                mTextEdit.setInputType(DEFAULT_INPUT_TYPE);

                FrameLayout.LayoutParams mEditTextLayoutParams = new FrameLayout.LayoutParams(
                    FrameLayout.LayoutParams.WRAP_CONTENT, FrameLayout.LayoutParams.WRAP_CONTENT);
                mTextEdit.setLayoutParams(mEditTextLayoutParams);
                addContentView(mTextEdit, mEditTextLayoutParams);

                // always place one character so all keyboards can send backspace
                mTextEdit.setText(" ");
                mTextEdit.setSelection(mTextEdit.getText().length());

                mTextEdit.addTextChangedListener(new TextWatcher() {
                    @Override
                    public void onTextChanged(CharSequence s, int start, int before, int count) {
                        if (ignoreKey) {
                            return;
                        }
                        if (count > 0) {
                            keyboardTyped(s.subSequence(start,start+count).toString());
                        }
                    }

                    @Override
                    public void beforeTextChanged(CharSequence s, int start, int count, int after) {
                        if (ignoreKey) {
                            return;
                        }
                        if (count > 0) {
                            for (int i = 0; i < count; i++) {
                                // send a backspace
                                keyboardDelete();
                            }
                        }
                    }

                    @Override
                    public void afterTextChanged(Editable s) {
                        // always place one character so all keyboards can send backspace
                        if (s.length() < 1) {
                            ignoreKey = true;
                            mTextEdit.setText(" ");
                            mTextEdit.setSelection(mTextEdit.getText().length());
                            ignoreKey = false;
                            return;
                        }
                    }
                });
            }
        });
	}

	@Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        // unhandled request
        if (requestCode != FILE_OPEN_CODE && requestCode != FILE_SAVE_CODE) {
            return;
        }

        // dialog was cancelled
        if (resultCode != Activity.RESULT_OK) {
            filePickerReturned("");
            return;
        }

        Uri uri = data.getData();
        filePickerReturned(uri.toString());
    }

    @Override
    public void onBackPressed() {
        if (goNativeActivity.keyboardUp) {
            hideKeyboard();
            return;
        }

        // skip the default behaviour - we can call finishActivity if we want to go back
        backPressed();
    }

    public void finishActivity() {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                GoNativeActivity.super.onBackPressed();
            }
        });
    }

    @Override
    public void onConfigurationChanged(Configuration config) {
        super.onConfigurationChanged(config);
        updateTheme(config);
    }

    protected void updateTheme(Configuration config) {
        boolean dark = (config.uiMode & Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES;
        setDarkMode(dark);
    }

    // -------------------------------------------------------------------------
    // Android accessibility bridge  (real-view overlay approach)
    // -------------------------------------------------------------------------

    static final int ROLE_BUTTON    = 1;
    static final int ROLE_TEXT      = 2;
    static final int ROLE_LINK      = 3;
    static final int ROLE_CONTAINER = 4;

    // Called once from Go (via JNI).
    static void setupAccessibility() {
        goNativeActivity.doSetupAccessibility();
    }

    void doSetupAccessibility() {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                if (mA11yContainer != null) {
                    return; // already set up
                }
                // Full-screen transparent container.  Its children are the real
                // accessibility Views; they intercept no touch events.
                mA11yContainer = new FrameLayout(goNativeActivity);
                mA11yContainer.setClickable(false);
                mA11yContainer.setFocusable(false);

                mA11yContainer.setImportantForAccessibility(
                        View.IMPORTANT_FOR_ACCESSIBILITY_AUTO);

                ViewGroup decorView = (ViewGroup) getWindow().getDecorView();
                decorView.addView(mA11yContainer, new FrameLayout.LayoutParams(
                        FrameLayout.LayoutParams.MATCH_PARENT,
                        FrameLayout.LayoutParams.MATCH_PARENT));

                // If Go already committed nodes before the UI thread ran, apply them now.
                applySnapshot();
            }
        });
    }

    // Called from Go (via JNI) before rebuilding the accessibility tree.
    static synchronized void clearAccessibilityNodes() {
        sStagingData.clear();
        sStagingLabels.clear();
    }

    // Called from Go (via JNI) to register one accessible element.
    static synchronized void addAccessibilityNode(int id, int role, String label,
            int x, int y, int width, int height, int parentID) {
        sStagingData.add(new int[]{id, role, x, y, width, height});
        sStagingLabels.add(label != null ? label : "");
    }

    // Called from Go (via JNI) after all nodes for this frame have been added.
    static synchronized void commitAccessibilityNodes() {
        // Build a cheap signature to avoid redundant UI work.
        StringBuilder sig = new StringBuilder();
        for (int i = 0; i < sStagingData.size(); i++) {
            int[] d = sStagingData.get(i);
            sig.append(d[0]).append(',').append(d[1]).append(',')
               .append(d[2]).append(',').append(d[3]).append(',')
               .append(d[4]).append(',').append(d[5]).append(';')
               .append(sStagingLabels.get(i)).append('|');
        }
        String newSig = sig.toString();
        boolean changed = !newSig.equals(sLastCommittedSignature);
        sLastCommittedSignature = newSig;

        if (!changed) {
            return;
        }

        // Snapshot the staging data for the UI thread (sStagingData is modified
        // on Go threads so we must not access it directly from the UI thread).
        final List<int[]>  snapData   = new ArrayList<>(sStagingData);
        final List<String> snapLabels = new ArrayList<>(sStagingLabels);

        if (mA11yContainer == null) {
            return; // UI not ready yet; doSetupAccessibility will call applySnapshot.
        }
        mA11yContainer.post(new Runnable() {
            @Override
            public void run() {
                rebuildA11yViews(snapData, snapLabels);
            }
        });
    }

    // Applies the most-recently committed snapshot.  Called on the UI thread
    // either from doSetupAccessibility (if setup ran after commit) or from the
    // post() in commitAccessibilityNodes.
    private static void applySnapshot() {
        // Take a consistent copy of the current staging data.
        final List<int[]>  snapData;
        final List<String> snapLabels;
        synchronized (GoNativeActivity.class) {
            snapData   = new ArrayList<>(sStagingData);
            snapLabels = new ArrayList<>(sStagingLabels);
        }
        rebuildA11yViews(snapData, snapLabels);
    }

    // Must be called on the UI thread.
    private static void rebuildA11yViews(List<int[]> data, List<String> labels) {
        if (mA11yContainer == null) {
            return;
        }
        mA11yContainer.removeAllViews();

        for (int i = 0; i < data.size(); i++) {
            final int[] d     = data.get(i);
            final String label = labels.get(i);
            final int role  = d[1];
            final int x = d[2], y = d[3], w = d[4], h = d[5];

            View v = new View(goNativeActivity);
            // Non-interactive so touch events fall through to the GL surface.
            // No background is set, so the view is visually transparent.
            // setAlpha is deliberately NOT called: alpha=0 causes TalkBack to
            // mark the view as not visible to the user and skip it.
            v.setClickable(false);
            v.setFocusable(false);
            v.setContentDescription(label);
            v.setImportantForAccessibility(View.IMPORTANT_FOR_ACCESSIBILITY_YES);

            // Customise the AccessibilityNodeInfo so TalkBack announces the
            // correct role (Button vs label) and actions.
            final int roleCopy = role;
            final String labelCopy = label;
            v.setAccessibilityDelegate(new View.AccessibilityDelegate() {
                @Override
                public void onInitializeAccessibilityNodeInfo(
                        View host, AccessibilityNodeInfo info) {
                    super.onInitializeAccessibilityNodeInfo(host, info);
                    info.setContentDescription(labelCopy);
                    switch (roleCopy) {
                        case ROLE_BUTTON:
                        case ROLE_LINK:
                            info.setClassName("android.widget.Button");
                            info.setClickable(true);
                            info.addAction(AccessibilityNodeInfo.ACTION_CLICK);
                            break;
                        default: // ROLE_TEXT
                            info.setClassName("android.widget.TextView");
                            info.setText(labelCopy);
                            break;
                    }
                }
            });

            // Position the view at the widget's screen-pixel coordinates.
            FrameLayout.LayoutParams lp = new FrameLayout.LayoutParams(w, h);
            lp.leftMargin = x;
            lp.topMargin  = y;
            mA11yContainer.addView(v, lp);
        }
    }
}
