-- Protected default API key PostgreSQL migration.
-- Target: PostgreSQL 15+. Safe to run repeatedly.

BEGIN;

SELECT pg_advisory_xact_lock(hashtext('molii-new-api-20260812-default-token'));

DO $migration$
BEGIN
  IF to_regclass('public.tokens') IS NOT NULL THEN
    ALTER TABLE public.tokens
      ADD COLUMN IF NOT EXISTS is_default boolean NOT NULL DEFAULT false;

    -- Deleted keys do not participate, so users may retain historical rows.
    CREATE UNIQUE INDEX IF NOT EXISTS ux_tokens_one_live_default_per_user
      ON public.tokens (user_id)
      WHERE is_default = true AND deleted_at IS NULL;
  END IF;
END
$migration$;

COMMIT;
