# Access policy extension

Owns registration proof v1, email suffix/alias/domain policy, regional page decisions,
and custom security configuration admission. Host services retain authentication,
settings persistence and atomic database guards. No host service or Ent imports.

## Disable contract

`access-policy` controls **new custom configuration writes**, not authorization.
When false, absent or unreadable, changes to domain quota, proof enabled/difficulty
and mainland page restriction are rejected. Unchanged or omitted fields are allowed;
native site settings and the upstream email whitelist remain editable. The frontend
omits locked fields rather than sending false and the backend checks both settings
save paths, so a stale form or direct request cannot bypass admission.

Persisted security rules, email alias uniqueness, atomic quota guards and old signed
v1 challenges remain enforced with the switch off. To stop an existing rule, an
administrator must explicitly disable it in security settings **while this module
is enabled**, then turn the module off. Switching off neither erases nor weakens rules.
Missing settings retain native defaults; failed reads and malformed persisted policy
return `ACCESS_POLICY_SETTINGS_UNAVAILABLE` (503), never an implicit allow.
Corrupt whitelist entries are rejected rather than silently discarded; administrators
can repair the native whitelist through the settings API without enabling this module.

## Page-only regional restriction

Pages render immediately as before pluginization. App still refreshes public settings
and applies the existing regional redirect, but settings latency or failure does not
introduce a full-page policy loading/error/retry screen. The Stage31 render boundary
was an unintended behavior change and was removed on 2026-10-04 at the user's request.
No new access strategy replaces it. Existing server-side rules and authentication are
unchanged; regional navigation is not a gateway/API access-control boundary.

## Homepage verification (2026-11-15)

`homepage_captcha_enabled` is an optional, default-off site-navigation gate, stored
in the existing settings table. It belongs to configuration admission; disabling
this extension does not disable an already configured gate. The prior regional
restriction behavior above remains unchanged.

When enabled, the backend returns a small standalone captcha document for `/`,
`/home`, `/home/` and `/index.html` before extension lookup and SPA serving. The
document contains a generated PNG, inline styles, and a small same-origin script;
it has no remote assets or homepage API calls. A successful form POST to
`/_homepage/verify` issues a 24-hour HttpOnly clearance cookie. HTTPS responses
also mark it Secure. Challenge and clearance tickets use separate authenticated
encryption from login credentials, and are bound to the browser user agent;
challenges additionally require a random HttpOnly browser cookie.

Form origin validation uses browser-generated `Sec-Fetch-Site: same-origin`
when present, so proxies rewriting Host to an internal upstream do not reject
legitimate HTTPS submissions. Cross-site and same-site sibling-domain submits
are rejected. Older clients fall back to comparing Origin with Host; preserve
the public Host at the proxy for those clients. Forwarded-Host alone is not
trusted. Origin rejection returns a readable 403 and
`X-Homepage-Verification-Error: origin_mismatch` for diagnosis.

Redis provides atomic single-use consumption and per-IP limits (30 new pages and
12 submissions per minute). Every submitted challenge is consumed, including a
wrong answer. Settings reads are cached for 30 seconds and refreshed on writes;
storage failures reject gated homepage access. The challenge page and cleared
homepage responses use `no-store` to avoid shared HTML cache bypasses.

This is a lightweight image challenge, not a replacement for upstream API
authentication, registration captcha or a WAF. It does not protect direct asset
URLs, login/admin recovery, gateway endpoints, or independent static hosting.
Homepage links inside the SPA return to the document server when the public
setting is enabled. Deployment must route both homepage and verification POSTs
to the backend and must not bypass it with an edge-cached homepage.

The Vite development server also checks the real backend before transforming
homepage HTML. It forwards the challenge and same-origin form POST, retaining
Host, Origin and cookies. Only an explicit backend 204 response with a `passed`
or `disabled` decision permits Vite to serve the app. The check header does not
bypass clearance checks. An unavailable or outdated backend produces 503 rather
than implicitly loading the app. Both backend and frontend changes are required.

## Adoption and verification

A standalone ledger migration adopts legacy installations as enabled and native
installations as disabled, preserves explicit values, and does not edit existing
rules. Historical SQL and migration order are unchanged. Module, service, handler,
migration, real SettingsView and App refresh-behavior tests cover on/off/missing/invalid
states, failures and retained rules. Local tests/builds are not PostgreSQL migration
rehearsal, live browser verification or production deployment.
