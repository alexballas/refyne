# Mobile Manifest/Plist Plan for Refyne and Go2TV

## Goal

Allow apps built with refyne to declare mobile platform capabilities, then let go2tv use those capabilities for reliable discovery, background casting, notifications, open-with/share, and durable mobile media access.

## Refyne Side

1. Add mobile platform config to `FyneApp.toml`.
   - Android: permissions, services, receivers, intent filters, activity attributes.
   - iOS: `Info.plist` keys, background modes, Bonjour services, document types, URL schemes.
   - iOS entitlements: associated domains and capability keys when needed.

2. Update packaging templates.
   - Merge app config into `AndroidManifest.xml`.
   - Merge app config into generated `Info.plist`.
   - Merge app config into generated iOS entitlements.
   - Validate duplicate or malformed declarations at package time.

3. Add focused runtime APIs.
   - Request runtime permissions, especially Android notifications.
   - Acquire/release Android multicast lock.
   - Start/stop Android foreground service for active work.
   - Deliver Android/iOS open-with, share, URL, and deep-link events to Go.
   - Support persistent Android URI grants and iOS security-scoped file access.

4. Add tests and examples.
   - Golden manifest/plist tests for common declarations.
   - Small mobile example app showing permissions, local network, and share intent handling.

## Go2TV Side

1. Add go2tv platform declarations.
   - Android: `CHANGE_WIFI_MULTICAST_STATE`, `POST_NOTIFICATIONS`, foreground-service permissions and service declaration.
   - iOS: `NSLocalNetworkUsageDescription`, `NSBonjourServices` for Chromecast/mDNS, suitable background modes only if Apple-compliant.
   - Android/iOS open-with/share declarations for media files and URLs.

2. Use new refyne runtime APIs.
   - Acquire multicast lock during SSDP/mDNS discovery.
   - Request notification permission before showing cast notifications.
   - Start a foreground service while serving local media or transcoding.
   - Stop the foreground service when casting stops.
   - Handle inbound file/URL/share events and prefill selected media.

3. Add durable mobile media state.
   - Persist Android URI permissions after file selection.
   - Use URI strings, not filesystem paths, for mobile queue and resume data.
   - Reopen media through `storage.Reader` or `storage.ReaderSeeker`.
   - Add mobile playlist and resume only after persistent file access is reliable.

4. Verify behavior.
   - Android: discovery works after app start, screen lock, and Wi-Fi changes.
   - Android: local cast/transcode survives app switch and screen lock.
   - Android: notification appears, opens go2tv, and disappears on stop.
   - Android/iOS: shared media URL opens go2tv ready to cast.
   - iOS: local network permission prompt is clear and discovery works after approval.

## Suggested Order

1. Refyne packaging config and golden tests.
2. Go2TV local-network declarations.
3. Refyne multicast-lock and notification-permission APIs.
4. Go2TV discovery lock and notification prompt.
5. Refyne foreground-service API.
6. Go2TV foreground service for active local casting/transcoding.
7. Refyne share/deep-link and persistent URI APIs.
8. Go2TV mobile open-with, queue, and resume.

## Notes

- Manifest/plist customization unlocks OS permissions but does not implement behavior by itself.
- Android is the stronger target for background local serving/transcoding because foreground services are supported.
- iOS background behavior must stay within Apple's allowed modes; do not assume arbitrary background HTTP serving is acceptable.
