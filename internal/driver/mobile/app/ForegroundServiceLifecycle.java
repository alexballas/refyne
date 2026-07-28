package org.golang.app;

// ForegroundServiceLifecycle serializes the two-step Android foreground-service
// startup contract with shutdown requests. startForegroundService() only asks
// the system to create the service; the service is not safely stoppable until
// onStartCommand() has called startForeground().
final class ForegroundServiceLifecycle {
    private boolean startPending;
    private boolean foreground;
    private boolean stopPending;
    private long startGeneration;

    synchronized long startRequested() {
        startGeneration++;
        stopPending = false;
        if (!foreground) {
            startPending = true;
        }
        return startGeneration;
    }

    synchronized void startFailed(long failedGeneration) {
        if (failedGeneration == startGeneration && !foreground) {
            startPending = false;
            stopPending = false;
        }
    }

    // deferStopIfStarting returns true when the caller must not call
    // stopService(). foregroundStarted() will consume the deferred request once
    // Android's startup contract has been satisfied.
    synchronized boolean deferStopIfStarting() {
        if (!startPending || foreground) {
            return false;
        }
        stopPending = true;
        return true;
    }

    // foregroundStarted marks the service safe to stop and returns whether a
    // stop arrived while startup was pending.
    synchronized boolean foregroundStarted() {
        startPending = false;
        foreground = true;

        boolean shouldStop = stopPending;
        stopPending = false;
        return shouldStop;
    }

    synchronized void stopped() {
        startPending = false;
        foreground = false;
        stopPending = false;
    }
}
