-- Molii New API PostgreSQL migration
-- Target: PostgreSQL 15+
-- Safe to run repeatedly. The application remains the canonical schema owner
-- and runs GORM AutoMigrate on the master node during startup.

BEGIN;

-- Serialize this migration when multiple deployment jobs start together.
SELECT pg_advisory_xact_lock(hashtext('molii-new-api-20260803'));

-- The Auto group feature stores an optional ordered JSON array in this text
-- column. Older New API databases do not have it. Fresh databases have no
-- tokens table yet, so the application will create the complete schema later.
DO $migration$
BEGIN
  IF to_regclass('public.tokens') IS NOT NULL THEN
    ALTER TABLE public.tokens
      ADD COLUMN IF NOT EXISTS auto_groups text;
  END IF;
END
$migration$;

COMMIT;
