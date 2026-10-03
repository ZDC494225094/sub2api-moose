# Media gateway extension

The runtime switch owns only new Gemini Veo `predictLongRunning` submissions. Native Grok images/video, Seedance, voice/realtime and Responses image tools never consult the flag. Existing task reads, cancellation and settlement retain host authorization and ownership checks.

Missing, invalid or unavailable flags reject new Veo work. The existing adoption SQL is immutable: legacy defaults true, native defaults false, explicit choices are retained. Module and handler tests cover native bypass and Veo fail-closed behavior. Local tests are not deployed upstream-network or PostgreSQL acceptance.
