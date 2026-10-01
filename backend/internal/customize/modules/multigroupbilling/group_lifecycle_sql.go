package multigroupbilling

// PostgreSQL statements preserve existing atomic bulk operations. Execution and
// transaction selection belong to the host repository. These are not migrations.
const ClearGroupBindingsSQL = `
			UPDATE api_keys ak
			SET group_id = CASE WHEN ak.group_id = $1 THEN NULL ELSE ak.group_id END,
				group_ids = COALESCE((
					SELECT jsonb_agg(value ORDER BY ord)
					FROM jsonb_array_elements(COALESCE(ak.group_ids, '[]'::jsonb)) WITH ORDINALITY AS items(value, ord)
					WHERE (value)::text::bigint <> $1
				), '[]'::jsonb),
				updated_at = NOW()
			WHERE ak.deleted_at IS NULL
			  AND (ak.group_id = $1 OR ak.group_ids @> $2::jsonb)
		`

const ReplaceGroupBindingsSQL = `
			WITH new_group AS (
				SELECT COALESCE(NULLIF(platform, ''), $5) AS platform
				FROM groups
				WHERE id = $3 AND deleted_at IS NULL
			)
			UPDATE api_keys ak
			SET group_id = CASE WHEN ak.group_id = $2 THEN $3 ELSE ak.group_id END,
				group_ids = COALESCE((
					SELECT jsonb_agg(to_jsonb(row_group_id) ORDER BY min_ord)
					FROM (
						SELECT row_group_id, MIN(ord) AS min_ord
						FROM (
							SELECT $3::bigint AS row_group_id, 0::bigint AS ord
							WHERE ak.group_id = $2
							UNION ALL
							SELECT CASE
								WHEN (value)::text::bigint = $2 THEN $3::bigint
								ELSE (value)::text::bigint
							END AS row_group_id,
							ord
							FROM jsonb_array_elements(COALESCE(ak.group_ids, '[]'::jsonb)) WITH ORDINALITY AS items(value, ord)
						) replaced
						WHERE row_group_id > 0
						GROUP BY row_group_id
					) deduped
				), '[]'::jsonb),
				platform = new_group.platform,
				updated_at = NOW()
			FROM new_group
			WHERE ak.user_id = $1
			  AND ak.deleted_at IS NULL
			  AND (ak.group_id = $2 OR ak.group_ids @> $4::jsonb)
		`
