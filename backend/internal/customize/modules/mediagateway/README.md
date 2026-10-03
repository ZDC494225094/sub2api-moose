# Media gateway extension

The runtime switch owns only new Gemini Veo `predictLongRunning` submissions. Native Grok images/video, Seedance, voice/realtime and Responses image tools never consult the flag. Existing task reads, cancellation and settlement retain host authorization and ownership checks.

Missing, invalid or unavailable flags reject new Veo work. The existing adoption SQL is immutable: legacy defaults true, native defaults false, explicit choices are retained. Module and handler tests cover native bypass and Veo fail-closed behavior. Local tests are not deployed upstream-network or PostgreSQL acceptance.

## Remaining ownership audit (2026-10-03)

Do not infer that every change in `service/grok_media.go` is upstream-native. Compared with `aea725f2`, this file adds the `grok-video-r` chat-completions adapter, compatible video status/URL extraction, synchronous-result billing and string image URL normalization. These differences still require individual ownership, admission and in-flight behavior review; the current switch does not cover all of them. The 13-managed catalog count is not evidence that this audit is complete.

The no-op native admission calls have been removed from Grok audio/media and Responses handlers, including the WebSocket per-message JSON classification. Contract checks prohibit reintroducing these calls. Only the genuine Gemini admission remains in those handlers.
