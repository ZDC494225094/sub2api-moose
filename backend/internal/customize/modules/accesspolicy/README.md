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

## Adoption and verification

A standalone ledger migration adopts legacy installations as enabled and native
installations as disabled, preserves explicit values, and does not edit existing
rules. Historical SQL and migration order are unchanged. Module, service, handler,
migration, real SettingsView and App refresh-behavior tests cover on/off/missing/invalid
states, failures and retained rules. Local tests/builds are not PostgreSQL migration
rehearsal, live browser verification or production deployment.
