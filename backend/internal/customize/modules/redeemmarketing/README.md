# Marketing Redeem Conversion Extension

Module ID: `redeem-marketing`. Managed by the existing customization switch page.
Legacy fork installations adopt it enabled; native installations default to
disabled. Explicit administrator choices are preserved by the migration ledger.

## Boundaries

- Domain policy and reporting SQL: this directory.
- UI, API adapter and page-local generation state:
  `frontend/src/extensions/modules/redeem-marketing/`.
- Table and setting migration:
  `internal/customize/migrations/sql/redeem-marketing/202610160001_batch_policy.sql`.
- Host adapters: dedicated `redeem_marketing_extension.go` files in service and
  admin handler packages. New routes inherit the existing admin authentication.
- Necessary host seams: two route registrations, two UI slots, one generation
  adapter, and one marketing-only eligibility call before redemption writes.
- No changes to native Ent schemas, generated Ent files or historical SQL.

This is not literally zero host changes: server-side redemption must consult
the immutable policy, and the existing page must mount the optional UI.

## Eligibility and Transactions

`new_users_only` means registration timestamp is greater than or equal to the
batch generation start timestamp, not "within the past N days" or "never paid".
The snapshot is immutable. Native/unrestricted and historical batches without
a policy retain their previous semantics. Per-user batch uniqueness is unchanged.

`POST /api/v1/admin/redeem-codes/marketing-batches/generate` accepts the native
marketing generation fields plus `new_users_only`. It runs the host generator
and policy insert in a single transaction under the existing idempotency helper.
Any code/policy failure rolls back the whole batch. The normal generator remains
available for unrestricted codes and other code types.

Turning the plugin off hides UI and denies new plugin generation/report calls.
It never removes restrictions from already-issued codes. No runtime uninstall or
table drop is safe while restricted codes remain outstanding.

## Report Definition

`GET /api/v1/admin/redeem-codes/marketing-batches` supports an exact `batch_id`,
`page`, and `page_size` (up to 100). Aggregation is limited to the requested page
of cohorts and uses parameterized SQL.

- Issued count: currently retained marketing codes in the batch.
- Redemption: persisted batch usages, one user per batch, and redeemed value.
- Conversion: distinct users with at least one qualifying subsequent payment
  divided by redeemed users; zero denominator is displayed as zero.
- Qualifying orders: created at/after redemption, paid strictly after it, status
  `PAID`, `RECHARGING`, or `COMPLETED`, positive `pay_amount`, zero `refund_amount`.
- Includes a user's first cash purchase after a promotional grant; it is not
  restricted to a second lifetime paid order. The UI labels it paid conversion.
- Actual cash is `pay_amount`, not credited `amount`; currency is normalized from
  `provider_snapshot.currency`, defaulting to CNY like operations finance.
- Each batch is an independent cohort. A user and later payment can contribute
  to multiple batches; do not sum across batches as unique revenue attribution.
- No fixed follow-up window: older cohorts have longer observation periods.
- Host code/user deletion can cascade to usage records and affect the report;
  this is a live report, not an immutable accounting ledger.

## Verification / Rollback

Tests cover cutoff equality, legacy unrestricted batches, failure-closed reads,
disabled admission, historical enforcement, atomic rollback, currency separation,
cohort deduplication, payment statuses, refunds, pre-redemption orders and paging.
SQLite SQL fixtures adapt PostgreSQL JSON/currency syntax only; they do not prove
PostgreSQL deployment or payment-provider acceptance.

For operational rollback, disable the plugin and retain its table and eligibility
hook. Back up the database before applying migrations in deployment. Do not
roll back to a binary without the eligibility hook while restricted codes exist.
