-- Existing keys retain their actual pre-switch behavior, even on installations
-- originally classified native which could create multigroup keys while pending.
-- Ownership is immutable for normal edits. New admission will choose an explicit
-- policy; never derive an existing key's policy from the current global toggle.
ALTER TABLE api_keys ADD COLUMN custom_routing_policy VARCHAR(32) NOT NULL DEFAULT 'multigroup-v1';
