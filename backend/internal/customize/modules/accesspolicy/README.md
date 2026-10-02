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

App requires a fresh public-settings request before mounting the page. Failure stays
at a retry boundary; server injection/cache is not a request-specific decision. A
restricted page cannot mount during the redirect interval. Native `/setup` remains
accessible for initialization. This remains a **page-only** rule: it does not turn
into a gateway/API regional blockade. It is not a replacement for backend auth.

## Adoption and verification

A standalone ledger migration adopts legacy installations as enabled and native
installations as disabled, preserves explicit values, and does not edit existing
rules. Historical SQL and migration order are unchanged. Module, service, handler,
migration, real SettingsView and App-boundary tests cover on/off/missing/invalid
states, failures and retained rules. Local tests/builds are not PostgreSQL migration
rehearsal, live browser verification or production deployment.
