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

    UPDATE public.models
    SET display_name = COALESCE(display_name, ''),
        description_en = COALESCE(description_en, ''),
        marketplace_enabled = COALESCE(marketplace_enabled, false),
        supported_parameters = COALESCE(supported_parameters, '[]'),
        supported_resolutions = COALESCE(supported_resolutions, '[]'),
        supported_aspect_ratios = COALESCE(supported_aspect_ratios, '[]'),
        max_input_images = COALESCE(max_input_images, 0),
        output_formats = COALESCE(output_formats, '[]'),
        min_duration = COALESCE(min_duration, 0),
        max_duration = COALESCE(max_duration, 0),
        reference_modalities = COALESCE(reference_modalities, '[]');

    ALTER TABLE public.models
      ALTER COLUMN display_name SET DEFAULT '',
      ALTER COLUMN display_name SET NOT NULL,
      ALTER COLUMN description_en SET DEFAULT '',
      ALTER COLUMN description_en SET NOT NULL,
      ALTER COLUMN marketplace_enabled SET DEFAULT false,
      ALTER COLUMN marketplace_enabled SET NOT NULL,
      ALTER COLUMN supported_parameters SET DEFAULT '[]',
      ALTER COLUMN supported_parameters SET NOT NULL,
      ALTER COLUMN supported_resolutions SET DEFAULT '[]',
      ALTER COLUMN supported_resolutions SET NOT NULL,
      ALTER COLUMN supported_aspect_ratios SET DEFAULT '[]',
      ALTER COLUMN supported_aspect_ratios SET NOT NULL,
      ALTER COLUMN max_input_images SET DEFAULT 0,
      ALTER COLUMN max_input_images SET NOT NULL,
      ALTER COLUMN output_formats SET DEFAULT '[]',
      ALTER COLUMN output_formats SET NOT NULL,
      ALTER COLUMN min_duration SET DEFAULT 0,
      ALTER COLUMN min_duration SET NOT NULL,
      ALTER COLUMN max_duration SET DEFAULT 0,
      ALTER COLUMN max_duration SET NOT NULL,
      ALTER COLUMN reference_modalities SET DEFAULT '[]',
      ALTER COLUMN reference_modalities SET NOT NULL;

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
