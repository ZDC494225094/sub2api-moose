# Media gateway extension (2026-10-04)

The switch owns new Veo `predictLongRunning`, the fork's OpenAI-compatible video adapter, `grok-video-r` text-to-video chat conversion, and official Grok string-image normalization. These features are absent from frozen upstream `aea725f2`. Native Grok image/video protocol, native Seedance, voice/realtime and Responses image tools do not consult the flag.

Missing/false flags reject new fork operations with 403; invalid/unavailable state rejects with 503. The host preserves these errors without retrying another account or reporting an upstream failure. Service admission precedes network submission. Existing result reads, URL/status normalization, cancellation and settlement remain ungated and retain host ownership checks; these completion helpers are not new-business admission points.

Do not gate every Grok request just because its service file has fork changes. The no-op native handler calls and WebSocket JSON classification remain removed and protected by contracts. Historical migration SQL is immutable: legacy defaults true, native false, explicit choices retained.

Service boundary tests cover each added adapter with off/missing/invalid state before network access; existing native forwarding tests continue without enabled fixtures, while fork success fixtures explicitly opt in. Handler/module tests cover Veo, native bypass and historical ownership. See the generated runtime evidence matrix for source/test locations. Local tests are not production PostgreSQL or real external-network acceptance.
