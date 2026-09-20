-- The custom API-key platform normalizer previously mapped newer upstream
-- platforms to anthropic. Align existing keys with their primary group now
-- that all upstream group platforms are supported by validation and routing.
UPDATE api_keys AS ak
SET platform = g.platform
FROM groups AS g
WHERE ak.group_id = g.id
  AND ak.deleted_at IS NULL
  AND ak.platform = 'anthropic'
  AND g.platform IN ('kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'composite');
