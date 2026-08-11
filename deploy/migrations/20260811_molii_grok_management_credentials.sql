-- Molii Grok management credential migration
-- Target: PostgreSQL 15+
-- Safe to run repeatedly. Fresh databases receive the same columns from the
-- application's GORM AutoMigrate after the base channels table is created.

BEGIN;

SELECT pg_advisory_xact_lock(hashtext('molii-new-api-20260811-grok-management-credentials'));

DO $migration$
BEGIN
  IF to_regclass('public.channels') IS NOT NULL THEN
    ALTER TABLE public.channels
      ADD COLUMN IF NOT EXISTS molii_grok_management_access_token text NOT NULL DEFAULT '',
      ADD COLUMN IF NOT EXISTS molii_grok_management_user_id integer NOT NULL DEFAULT 0;
  END IF;
END
$migration$;

COMMIT;
