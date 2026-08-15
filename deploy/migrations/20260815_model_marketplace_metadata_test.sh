#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(CDPATH= cd -- "$script_dir/../.." && pwd)
migration="$script_dir/20260815_model_marketplace_metadata.sql"
postgres_image=${POSTGRES_TEST_IMAGE:-postgres:15-alpine@sha256:3d0f7584ed7d04e27fa050d6683a74746608faf21f202be78460d679cc56461f}
container="model-marketplace-metadata-migration-test-$$"

if [ ! -f "$migration" ]; then
  echo "missing migration: $migration" >&2
  exit 1
fi

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM

docker run --detach --rm --name "$container" \
  --publish 127.0.0.1::5432 \
  --env POSTGRES_DB=migration_contract \
  --env POSTGRES_PASSWORD=migration_contract \
  --env POSTGRES_USER=migration_contract \
  "$postgres_image" >/dev/null

attempt=0
until docker exec "$container" pg_isready --username=migration_contract --dbname=migration_contract >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    docker logs "$container" >&2
    exit 1
  fi
  sleep 1
done

docker exec --interactive "$container" psql --username=migration_contract --dbname=migration_contract --set=ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE public.models (
  id bigserial PRIMARY KEY,
  model_name varchar(128) NOT NULL,
  status bigint NOT NULL DEFAULT 1,
  deleted_at timestamptz
);
INSERT INTO public.models (model_name) VALUES ('catalog-model');
SQL

run_migration() {
  docker exec --interactive "$container" psql \
    --username=migration_contract --dbname=migration_contract \
    --set=ON_ERROR_STOP=1 <"$migration" >/dev/null
}

run_migration
run_migration

docker exec --interactive "$container" psql --username=migration_contract --dbname=migration_contract --set=ON_ERROR_STOP=1 <<'SQL'
DO $contract$
DECLARE
  column_contract record;
BEGIN
  FOR column_contract IN
    SELECT * FROM (VALUES
      ('display_name', 'character varying'),
      ('description_en', 'text'),
      ('marketplace_enabled', 'boolean'),
      ('supported_parameters', 'text'),
      ('supported_resolutions', 'text'),
      ('supported_aspect_ratios', 'text'),
      ('max_input_images', 'bigint'),
      ('output_formats', 'text'),
      ('min_duration', 'bigint'),
      ('max_duration', 'bigint'),
      ('reference_modalities', 'text')
    ) AS expected(column_name, data_type)
  LOOP
    IF NOT EXISTS (
      SELECT 1
      FROM information_schema.columns
      WHERE table_schema = 'public'
        AND table_name = 'models'
        AND column_name = column_contract.column_name
        AND data_type = column_contract.data_type
        AND is_nullable = 'NO'
    ) THEN
      RAISE EXCEPTION 'models.% contract mismatch', column_contract.column_name;
    END IF;
  END LOOP;

  IF NOT EXISTS (
    SELECT 1
    FROM pg_indexes
    WHERE schemaname = 'public'
      AND tablename = 'models'
      AND indexname = 'idx_models_marketplace_enabled_status'
      AND indexdef LIKE '%(marketplace_enabled, status)%'
      AND indexdef LIKE '%WHERE (deleted_at IS NULL)%'
  ) THEN
    RAISE EXCEPTION 'marketplace publication index is missing or malformed';
  END IF;

  IF (SELECT display_name FROM public.models WHERE model_name = 'catalog-model') <> 'catalog-model' THEN
    RAISE EXCEPTION 'blank display_name was not backfilled';
  END IF;
END
$contract$;

INSERT INTO public.models (model_name) VALUES ('default-model');
DO $contract$
DECLARE
  defaults public.models%ROWTYPE;
BEGIN
  SELECT * INTO defaults FROM public.models WHERE model_name = 'default-model';
  IF defaults.display_name <> '' OR defaults.description_en <> '' OR defaults.marketplace_enabled <> false OR
     defaults.supported_parameters <> '[]' OR defaults.supported_resolutions <> '[]' OR
     defaults.supported_aspect_ratios <> '[]' OR defaults.max_input_images <> 0 OR
     defaults.output_formats <> '[]' OR defaults.min_duration <> 0 OR defaults.max_duration <> 0 OR
     defaults.reference_modalities <> '[]' THEN
    RAISE EXCEPTION 'model marketplace metadata defaults mismatch';
  END IF;
END
$contract$;
SQL

docker exec "$container" createdb --username=migration_contract fresh_bootstrap
docker exec "$container" createdb --username=migration_contract concurrent_backfill
docker exec --interactive "$container" psql \
  --username=migration_contract --dbname=fresh_bootstrap \
  --set=ON_ERROR_STOP=1 <"$migration" >/dev/null
docker exec --interactive "$container" psql --username=migration_contract --dbname=fresh_bootstrap --set=ON_ERROR_STOP=1 <<'SQL'
DO $contract$
BEGIN
  IF to_regclass('public.models') IS NOT NULL THEN
    RAISE EXCEPTION 'fresh Compose migration ordering fixture unexpectedly created models';
  END IF;
END
$contract$;
SQL

postgres_port=$(docker port "$container" 5432/tcp | sed -n '1s/.*://p')
if [ -z "$postgres_port" ]; then
  echo "failed to resolve PostgreSQL test port" >&2
  exit 1
fi
(
  cd "$project_dir"
  MODEL_MARKETPLACE_POSTGRES_TEST_DSN="postgresql://migration_contract:migration_contract@127.0.0.1:$postgres_port/fresh_bootstrap?sslmode=disable" \
    go test ./model -run 'TestMarketplacePostgresFreshBootstrapConverges' -count=1
  MODEL_MARKETPLACE_POSTGRES_TEST_DSN="postgresql://migration_contract:migration_contract@127.0.0.1:$postgres_port/concurrent_backfill?sslmode=disable" \
    go test ./model -run 'TestBackfillLocalMarketplaceMetadataPreservesConcurrentAdministratorUpdate' -count=1
)

echo "Model marketplace metadata migration contract passed"
