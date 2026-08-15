-- Local model marketplace metadata PostgreSQL migration.
-- Target: PostgreSQL 15+. Safe to run repeatedly.

BEGIN;

SELECT pg_advisory_xact_lock(hashtext('molii-new-api-20260815-model-marketplace-metadata'));

DO $migration$
BEGIN
  IF to_regclass('public.models') IS NOT NULL THEN
    ALTER TABLE public.models
      ADD COLUMN IF NOT EXISTS display_name varchar(255) NOT NULL DEFAULT '',
      ADD COLUMN IF NOT EXISTS description_en text NOT NULL DEFAULT '',
      ADD COLUMN IF NOT EXISTS marketplace_enabled boolean NOT NULL DEFAULT false,
      ADD COLUMN IF NOT EXISTS supported_parameters text NOT NULL DEFAULT '[]',
      ADD COLUMN IF NOT EXISTS supported_resolutions text NOT NULL DEFAULT '[]',
      ADD COLUMN IF NOT EXISTS supported_aspect_ratios text NOT NULL DEFAULT '[]',
      ADD COLUMN IF NOT EXISTS max_input_images bigint NOT NULL DEFAULT 0,
      ADD COLUMN IF NOT EXISTS output_formats text NOT NULL DEFAULT '[]',
      ADD COLUMN IF NOT EXISTS min_duration bigint NOT NULL DEFAULT 0,
      ADD COLUMN IF NOT EXISTS max_duration bigint NOT NULL DEFAULT 0,
      ADD COLUMN IF NOT EXISTS reference_modalities text NOT NULL DEFAULT '[]';

    CREATE INDEX IF NOT EXISTS idx_models_marketplace_enabled_status
      ON public.models (marketplace_enabled, status)
      WHERE deleted_at IS NULL;

    UPDATE public.models
    SET display_name = model_name
    WHERE COALESCE(display_name, '') = '';
  END IF;
END
$migration$;

COMMIT;
