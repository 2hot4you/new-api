-- Marketplace display-order PostgreSQL migration.
-- Target: PostgreSQL 15+. Safe to run repeatedly.

BEGIN;

SELECT pg_advisory_xact_lock(hashtext('molii-new-api-20260820-marketplace-display-order'));

CREATE TABLE IF NOT EXISTS public.marketplace_order_locks (
  name varchar(64) PRIMARY KEY,
  version bigint NOT NULL DEFAULT 0
);

INSERT INTO public.marketplace_order_locks (name, version)
VALUES ('marketplace_display_order', 0)
ON CONFLICT (name) DO NOTHING;

SELECT name
FROM public.marketplace_order_locks
WHERE name = 'marketplace_display_order'
FOR UPDATE;

DO $migration$
BEGIN
  IF to_regclass('public.models') IS NOT NULL THEN
    ALTER TABLE public.models
      ADD COLUMN IF NOT EXISTS display_order bigint NOT NULL DEFAULT 0;

    CREATE INDEX IF NOT EXISTS idx_models_display_order
      ON public.models (display_order);

    WITH stats AS (
      SELECT COALESCE(MAX(display_order) FILTER (WHERE display_order > 0), 0)::bigint AS max_order,
             COUNT(*) FILTER (WHERE display_order <= 0)::bigint AS unset_count
      FROM public.models
      WHERE deleted_at IS NULL
    ),
    available_orders AS (
      SELECT candidate,
             ROW_NUMBER() OVER (ORDER BY candidate) AS position
      FROM stats,
           generate_series(1::bigint, stats.max_order + stats.unset_count) AS candidate
      WHERE NOT EXISTS (
        SELECT 1
        FROM public.models used
        WHERE used.deleted_at IS NULL
          AND used.display_order > 0
          AND used.display_order = candidate
      )
    ),
    ranked_models AS (
      SELECT id,
             ROW_NUMBER() OVER (
               ORDER BY
                 CASE
                   WHEN release_date ~ '^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$'
                     THEN CASE WHEN to_char(to_date(release_date, 'YYYY-MM-DD'), 'YYYY-MM-DD') = release_date THEN 0 ELSE 1 END
                   ELSE 1
                 END,
                 CASE
                   WHEN release_date ~ '^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$'
                     THEN CASE WHEN to_char(to_date(release_date, 'YYYY-MM-DD'), 'YYYY-MM-DD') = release_date THEN release_date END
                 END DESC,
                 model_name ASC,
                 id ASC
             ) AS position
      FROM public.models
      WHERE deleted_at IS NULL
        AND display_order <= 0
    )
    UPDATE public.models target
    SET display_order = available_orders.candidate
    FROM ranked_models
    JOIN available_orders USING (position)
    WHERE target.id = ranked_models.id
      AND target.display_order <= 0;
  END IF;

  IF to_regclass('public.vendors') IS NOT NULL THEN
    ALTER TABLE public.vendors
      ADD COLUMN IF NOT EXISTS display_order bigint NOT NULL DEFAULT 0;

    CREATE INDEX IF NOT EXISTS idx_vendors_display_order
      ON public.vendors (display_order);

    WITH stats AS (
      SELECT COALESCE(MAX(display_order) FILTER (WHERE display_order > 0), 0)::bigint AS max_order,
             COUNT(*) FILTER (WHERE display_order <= 0)::bigint AS unset_count
      FROM public.vendors
      WHERE deleted_at IS NULL
    ),
    available_orders AS (
      SELECT candidate,
             ROW_NUMBER() OVER (ORDER BY candidate) AS position
      FROM stats,
           generate_series(1::bigint, stats.max_order + stats.unset_count) AS candidate
      WHERE NOT EXISTS (
        SELECT 1
        FROM public.vendors used
        WHERE used.deleted_at IS NULL
          AND used.display_order > 0
          AND used.display_order = candidate
      )
    ),
    ranked_vendors AS (
      SELECT id,
             ROW_NUMBER() OVER (ORDER BY id ASC) AS position
      FROM public.vendors
      WHERE deleted_at IS NULL
        AND display_order <= 0
    )
    UPDATE public.vendors target
    SET display_order = available_orders.candidate
    FROM ranked_vendors
    JOIN available_orders USING (position)
    WHERE target.id = ranked_vendors.id
      AND target.display_order <= 0;
  END IF;
END
$migration$;

COMMIT;
