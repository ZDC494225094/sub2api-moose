-- Billing-scheduling extension migration marker
-- This migration marks billing and scheduling enhancement features as belonging
-- to the billing-scheduling extension module.

-- Extension features controlled:
-- 1. Rate Multiplier (分组倍率同步)
--    - user_group_rates table: custom rate multipliers per group
--    - scheduler_cache: rate_multiplier fields
-- 2. Time-based Pricing (渠道分时定价)
--    - channel pricing with time-based rules
--    - pricing_service: time-aware pricing logic
-- 3. Custom Usage (自定义分组用量)
--    - custom usage calculation rules
--    - usage aggregation logic
-- 4. Composite Scheduling (组合调度)
--    - complex scheduling strategies
--    - multi-factor routing decisions

-- Extension control behavior:
-- When billing-scheduling extension is DISABLED:
--   1. Rate multipliers fall back to 1.0 (no multiplier enhancement)
--   2. Time-based pricing falls back to standard pricing (no time differentiation)
--   3. Custom usage calculations fall back to standard usage counting
--   4. Composite scheduling falls back to simple round-robin or random
--   5. **Historical data is NEVER modified** - existing usage and billing records stay unchanged

-- When billing-scheduling extension is ENABLED:
--   All enhanced billing and scheduling features are available.

-- CRITICAL CONSTRAINT: Historical Data Immutability
-- Existing usage records, billing records, and cached billing data MUST NOT be
-- recalculated or modified when the extension is toggled. The extension only affects
-- NEW operations going forward. This is enforced at the service layer, not the database.

-- Related tables (existing, not modified here):
--   - user_group_rates: rate multipliers per group
--   - scheduler_cache: scheduling state with rate info
--   - usage_records: historical usage (immutable)
--   - billing_records: historical billing (immutable)
--   - channels: pricing configuration
--   - pricing rules: time-based pricing data

-- No schema changes needed; this is a marker for extension ownership.
-- Extension metadata:
--   module: billing-scheduling
--   tables: user_group_rates, scheduler_cache (rate fields)
--   features: rate_multiplier, time_pricing, custom_usage, composite_scheduling
--   native: false (二开功能，上游不存在)
--   constraint: historical_data_immutable
