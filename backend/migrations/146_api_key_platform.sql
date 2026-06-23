-- Add vendor/platform boundary to API keys for same-platform group selection.

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS platform VARCHAR(32) NOT NULL DEFAULT 'anthropic';

UPDATE api_keys ak
SET platform = g.platform
FROM groups g
WHERE ak.group_id = g.id
  AND g.platform IS NOT NULL
  AND g.platform <> '';

UPDATE api_keys ak
SET platform = g.platform
FROM groups g
WHERE ak.group_id IS NULL
  AND jsonb_typeof(ak.group_ids) = 'array'
  AND jsonb_array_length(ak.group_ids) > 0
  AND g.id = (ak.group_ids->>0)::bigint
  AND g.platform IS NOT NULL
  AND g.platform <> '';

CREATE INDEX IF NOT EXISTS api_keys_platform
    ON api_keys(platform)
    WHERE deleted_at IS NULL;
