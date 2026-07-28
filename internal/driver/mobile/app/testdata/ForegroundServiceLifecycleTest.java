package org.golang.app;

public final class ForegroundServiceLifecycleTest {
    public static void main(String[] args) {
        stopDuringStartupIsDeferred();
        foregroundServiceStopsImmediately();
        failedStartClearsDeferredStop();
        repeatedStartWhileForegroundRemainsStoppable();
        newerStartCancelsDeferredStop();
        staleFailureDoesNotClearNewerStart();
    }

    private static void stopDuringStartupIsDeferred() {
        ForegroundServiceLifecycle lifecycle = new ForegroundServiceLifecycle();
        lifecycle.startRequested();

        check(lifecycle.deferStopIfStarting(), "startup stop was not deferred");
        check(lifecycle.foregroundStarted(), "deferred stop was not consumed after promotion");
    }

    private static void foregroundServiceStopsImmediately() {
        ForegroundServiceLifecycle lifecycle = new ForegroundServiceLifecycle();
        lifecycle.startRequested();

        check(!lifecycle.foregroundStarted(), "normal promotion invented a stop");
        check(!lifecycle.deferStopIfStarting(), "foreground stop was incorrectly deferred");
    }

    private static void failedStartClearsDeferredStop() {
        ForegroundServiceLifecycle lifecycle = new ForegroundServiceLifecycle();
        long generation = lifecycle.startRequested();
        check(lifecycle.deferStopIfStarting(), "startup stop was not deferred");

        lifecycle.startFailed(generation);
        check(!lifecycle.deferStopIfStarting(), "failed start remained pending");
    }

    private static void repeatedStartWhileForegroundRemainsStoppable() {
        ForegroundServiceLifecycle lifecycle = new ForegroundServiceLifecycle();
        lifecycle.startRequested();
        lifecycle.foregroundStarted();
        lifecycle.startRequested();

        check(!lifecycle.deferStopIfStarting(), "repeat start made a foreground service pending");
    }

    private static void newerStartCancelsDeferredStop() {
        ForegroundServiceLifecycle lifecycle = new ForegroundServiceLifecycle();
        lifecycle.startRequested();
        check(lifecycle.deferStopIfStarting(), "startup stop was not deferred");

        lifecycle.startRequested();
        check(!lifecycle.foregroundStarted(), "newer start retained the older deferred stop");
    }

    private static void staleFailureDoesNotClearNewerStart() {
        ForegroundServiceLifecycle lifecycle = new ForegroundServiceLifecycle();
        long oldGeneration = lifecycle.startRequested();
        lifecycle.startRequested();

        lifecycle.startFailed(oldGeneration);
        check(lifecycle.deferStopIfStarting(), "stale failure cleared the newer start");
    }

    private static void check(boolean condition, String message) {
        if (!condition) {
            throw new AssertionError(message);
        }
    }
}
