# Subscription issuance extension

`subscription-extensions` owns immutable issuance policy and schema contributions. The host keeps transaction orchestration, repositories, expiry, quotas, billing and historical subscription selection. Disabling the extension cannot erase old entitlements.

## Durable policy

- Enabled: independent-v1 grants a new independent subscription for each issuance.
- Disabled/missing flag: upstream-v1 uses a native-owned single lineage per user/group. Administrative assignment is idempotent (conflicting terms rejected); paid purchases/redemptions renew that lineage. Existing independent grants are never selected as native renewal targets.
- Invalid/unavailable live settings fail closed. The compatibility constructor defaults to native behavior.
- Policy is frozen at order/code creation, once per bulk assignment or code batch. Orders store custom_subscription_policy in provider snapshots; redeem codes and subscriptions persist the immutable column. Fulfillment reads that frozen policy, never the live flag. Historical missing snapshots/columns resolve to independent-v1; malformed explicit snapshots are rejected.
- Native issuance locks the existing user row FOR UPDATE before looking up/creating the native lineage, within the same Ent transaction as renewal/payment/redemption. No global user/group uniqueness is reintroduced. Post-commit cache invalidation failures do not falsely report an already committed grant as failed.

Old multi-subscription quotas, precise expiry, task usage, deletion cache eviction and read-only order/code labels remain host compatibility behavior. Homepage display_purchase_count belongs to premium-home presentation (hidden with that route); shared plan configuration is retained, not a second issuance switch. Existing entitlement pages remain visible.

## Adoption and verification

The additive ledger migration defaults historical rows to independent-v1, adds a non-unique lookup index, adopts legacy/native defaults and preserves explicit/deleted switches. Ent mixes the schema contribution from entschema/issuance.go; generated files must be regenerated together.

Module tests cover defaults and immutable snapshots; service tests cover native idempotence/renewal, independent compatibility, bulk freezing and actual order/code fulfillment after the switch changes. sqlmock checks lock ordering/transaction requirements; SQLite tests check migration rollback and retained duplicate grants. These do not prove PostgreSQL concurrent first-issuance, real payment callbacks, production migration or browser acceptance.
