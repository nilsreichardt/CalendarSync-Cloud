UPDATE users
SET email = lower(btrim(email)), updated_at = NOW()
WHERE email <> lower(btrim(email));

DO $$
DECLARE
  rec RECORD;
  canonical_user_id UUID;
  duplicate_user_ids UUID[];
BEGIN
  FOR rec IN
    SELECT lower(btrim(email)) AS normalized_email
    FROM users
    GROUP BY 1
    HAVING count(*) > 1
  LOOP
    SELECT id
    INTO canonical_user_id
    FROM users
    WHERE lower(btrim(email)) = rec.normalized_email
    ORDER BY created_at ASC, id ASC
    LIMIT 1;

    SELECT COALESCE(array_agg(id), ARRAY[]::UUID[])
    INTO duplicate_user_ids
    FROM (
      SELECT id
      FROM users
      WHERE lower(btrim(email)) = rec.normalized_email
        AND id <> canonical_user_id
      ORDER BY created_at ASC, id ASC
    ) AS duplicates;

    IF array_length(duplicate_user_ids, 1) IS NULL THEN
      CONTINUE;
    END IF;

    UPDATE sync_rules
    SET user_id = canonical_user_id
    WHERE user_id = ANY(duplicate_user_ids);

    INSERT INTO connection_calendars (connection_id, calendar_id, summary, is_primary, access_role, created_at)
    SELECT existing.id, cc.calendar_id, cc.summary, cc.is_primary, cc.access_role, cc.created_at
    FROM google_connections duplicate_gc
    INNER JOIN google_connections existing
      ON existing.user_id = canonical_user_id
     AND existing.google_sub = duplicate_gc.google_sub
    INNER JOIN connection_calendars cc ON cc.connection_id = duplicate_gc.id
    WHERE duplicate_gc.user_id = ANY(duplicate_user_ids)
    ON CONFLICT (connection_id, calendar_id) DO UPDATE SET
      summary = EXCLUDED.summary,
      is_primary = connection_calendars.is_primary OR EXCLUDED.is_primary,
      access_role = CASE
        WHEN connection_calendars.access_role = '' THEN EXCLUDED.access_role
        ELSE connection_calendars.access_role
      END;

    INSERT INTO encrypted_oauth_tokens (
      connection_id, provider, cipher_text, dek_cipher_text, nonce, key_version, created_at, updated_at
    )
    SELECT existing.id, token.provider, token.cipher_text, token.dek_cipher_text, token.nonce, token.key_version, NOW(), NOW()
    FROM google_connections duplicate_gc
    INNER JOIN google_connections existing
      ON existing.user_id = canonical_user_id
     AND existing.google_sub = duplicate_gc.google_sub
    INNER JOIN encrypted_oauth_tokens token ON token.connection_id = duplicate_gc.id
    WHERE duplicate_gc.user_id = ANY(duplicate_user_ids)
    ON CONFLICT (connection_id) DO UPDATE SET
      provider = EXCLUDED.provider,
      cipher_text = EXCLUDED.cipher_text,
      dek_cipher_text = EXCLUDED.dek_cipher_text,
      nonce = EXCLUDED.nonce,
      key_version = EXCLUDED.key_version,
      updated_at = NOW();

    UPDATE sync_rules AS sr
    SET source_connection_id = existing.id
    FROM google_connections duplicate_gc
    INNER JOIN google_connections existing
      ON existing.user_id = canonical_user_id
     AND existing.google_sub = duplicate_gc.google_sub
    WHERE duplicate_gc.user_id = ANY(duplicate_user_ids)
      AND sr.source_connection_id = duplicate_gc.id;

    UPDATE sync_rules AS sr
    SET target_connection_id = existing.id
    FROM google_connections duplicate_gc
    INNER JOIN google_connections existing
      ON existing.user_id = canonical_user_id
     AND existing.google_sub = duplicate_gc.google_sub
    WHERE duplicate_gc.user_id = ANY(duplicate_user_ids)
      AND sr.target_connection_id = duplicate_gc.id;

    UPDATE google_connections AS gc
    SET user_id = canonical_user_id, updated_at = NOW()
    WHERE gc.user_id = ANY(duplicate_user_ids)
      AND NOT EXISTS (
        SELECT 1
        FROM google_connections existing
        WHERE existing.user_id = canonical_user_id
          AND existing.google_sub = gc.google_sub
      );

    DELETE FROM google_connections AS gc
    WHERE gc.user_id = ANY(duplicate_user_ids)
      AND EXISTS (
        SELECT 1
        FROM google_connections existing
        WHERE existing.user_id = canonical_user_id
          AND existing.google_sub = gc.google_sub
      );

    DELETE FROM users
    WHERE id = ANY(duplicate_user_ids);

    UPDATE users
    SET email = rec.normalized_email, updated_at = NOW()
    WHERE id = canonical_user_id;
  END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS users_email_normalized_unique_idx
ON users ((lower(btrim(email))));
