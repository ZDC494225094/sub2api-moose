-- An admin delete of a stuck bundle is recorded first. Only the node that
-- holds the local file removes it and then the index row, so no node ever
-- deletes files merely because their rows are missing.
ALTER TABLE gateway_log_archive_bundles ADD COLUMN delete_requested_at TIMESTAMPTZ;
