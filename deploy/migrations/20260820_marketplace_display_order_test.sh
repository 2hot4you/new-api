#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
migration="$script_dir/20260820_marketplace_display_order.sql"
postgres_image=${POSTGRES_TEST_IMAGE:-postgres:15-alpine@sha256:3d0f7584ed7d04e27fa050d6683a74746608faf21f202be78460d679cc56461f}
container="marketplace-display-order-migration-test-$$"

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
ready_checks=0
while [ "$ready_checks" -lt 3 ]; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    docker logs "$container" >&2
    exit 1
  fi
  if docker exec "$container" pg_isready --username=migration_contract --dbname=migration_contract >/dev/null 2>&1; then
    ready_checks=$((ready_checks + 1))
  else
    ready_checks=0
  fi
  sleep 1
done

docker exec --interactive "$container" psql --username=migration_contract --dbname=migration_contract --set=ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE public.models (
  id bigserial PRIMARY KEY,
  model_name varchar(128) NOT NULL,
  release_date varchar(32),
  deleted_at timestamptz
);
CREATE TABLE public.vendors (
  id bigserial PRIMARY KEY,
  name varchar(128) NOT NULL,
  deleted_at timestamptz
);

INSERT INTO public.models (model_name, release_date) VALUES
  ('older', '2026-01-01'),
  ('newer', '2026-08-01'),
  ('invalid-b', 'August 2026'),
  ('invalid-a', NULL),
  ('deleted', '2027-01-01');
UPDATE public.models SET deleted_at = now() WHERE model_name = 'deleted';

INSERT INTO public.vendors (name) VALUES ('first'), ('pinned'), ('third'), ('deleted');
UPDATE public.vendors SET deleted_at = now() WHERE name = 'deleted';
SQL

run_migration() {
  docker exec --interactive "$container" psql \
    --username=migration_contract --dbname=migration_contract \
    --set=ON_ERROR_STOP=1 <"$migration" >/dev/null
}

run_migration

docker exec --interactive "$container" psql --username=migration_contract --dbname=migration_contract --set=ON_ERROR_STOP=1 <<'SQL'
UPDATE public.models SET display_order = 7 WHERE model_name = 'invalid-b';
UPDATE public.vendors SET display_order = 6 WHERE name = 'third';
UPDATE public.models SET display_order = 0 WHERE model_name IN ('older', 'newer', 'invalid-a');
UPDATE public.vendors SET display_order = 0 WHERE name IN ('first', 'pinned');
SQL

run_migration

before=$(docker exec "$container" psql --username=migration_contract --dbname=migration_contract --tuples-only --no-align --command="
  SELECT string_agg('m:' || id || ':' || display_order, ',' ORDER BY id) FROM public.models;
  SELECT string_agg('v:' || id || ':' || display_order, ',' ORDER BY id) FROM public.vendors;
")

run_migration

after=$(docker exec "$container" psql --username=migration_contract --dbname=migration_contract --tuples-only --no-align --command="
  SELECT string_agg('m:' || id || ':' || display_order, ',' ORDER BY id) FROM public.models;
  SELECT string_agg('v:' || id || ':' || display_order, ',' ORDER BY id) FROM public.vendors;
")

if [ "$before" != "$after" ]; then
  echo "second migration execution changed display order" >&2
  exit 1
fi

docker exec --interactive "$container" psql --username=migration_contract --dbname=migration_contract --set=ON_ERROR_STOP=1 <<'SQL'
DO $contract$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'models' AND column_name = 'display_order'
      AND data_type = 'bigint' AND is_nullable = 'NO' AND column_default = '0'
  ) THEN
    RAISE EXCEPTION 'models.display_order column contract mismatch';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'vendors' AND column_name = 'display_order'
      AND data_type = 'bigint' AND is_nullable = 'NO' AND column_default = '0'
  ) THEN
    RAISE EXCEPTION 'vendors.display_order column contract mismatch';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes
    WHERE schemaname = 'public' AND tablename = 'models'
      AND indexname = 'idx_models_display_order' AND indexdef LIKE '%(display_order)%'
  ) THEN
    RAISE EXCEPTION 'models display-order index missing';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes
    WHERE schemaname = 'public' AND tablename = 'vendors'
      AND indexname = 'idx_vendors_display_order' AND indexdef LIKE '%(display_order)%'
  ) THEN
    RAISE EXCEPTION 'vendors display-order index missing';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'marketplace_order_locks' AND column_name = 'name'
      AND data_type = 'character varying' AND character_maximum_length = 64 AND is_nullable = 'NO'
  ) OR NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'marketplace_order_locks' AND column_name = 'version'
      AND data_type = 'bigint' AND is_nullable = 'NO' AND column_default = '0'
  ) THEN
    RAISE EXCEPTION 'marketplace order lock table contract mismatch';
  END IF;
  IF (SELECT COUNT(*) FROM public.marketplace_order_locks WHERE name = 'marketplace_display_order') <> 1 THEN
    RAISE EXCEPTION 'marketplace order lock row is missing or duplicated';
  END IF;
  IF EXISTS (SELECT 1 FROM public.models WHERE deleted_at IS NULL AND display_order <= 0) THEN
    RAISE EXCEPTION 'active models were not assigned positive order';
  END IF;
  IF EXISTS (SELECT 1 FROM public.vendors WHERE deleted_at IS NULL AND display_order <= 0) THEN
    RAISE EXCEPTION 'active vendors were not assigned positive order';
  END IF;
  IF (SELECT display_order FROM public.models WHERE model_name = 'invalid-b') <> 7 THEN
    RAISE EXCEPTION 'positive model order was not preserved';
  END IF;
  IF (SELECT display_order FROM public.vendors WHERE name = 'third') <> 6 THEN
    RAISE EXCEPTION 'positive vendor order was not preserved';
  END IF;
  IF (SELECT display_order FROM public.models WHERE model_name = 'newer') >=
     (SELECT display_order FROM public.models WHERE model_name = 'older') THEN
    RAISE EXCEPTION 'models were not ordered by release date descending';
  END IF;
  IF (SELECT display_order FROM public.vendors WHERE name = 'first') >=
     (SELECT display_order FROM public.vendors WHERE name = 'pinned') THEN
    RAISE EXCEPTION 'vendors were not ordered by id ascending';
  END IF;
  IF (SELECT display_order FROM public.models WHERE model_name = 'deleted') <> 0 OR
     (SELECT display_order FROM public.vendors WHERE name = 'deleted') <> 0 THEN
    RAISE EXCEPTION 'soft-deleted rows should not be backfilled';
  END IF;
END
$contract$;

INSERT INTO public.models (model_name) VALUES ('default-model');
INSERT INTO public.vendors (name) VALUES ('default-vendor');
DO $contract$
BEGIN
  IF (SELECT display_order FROM public.models WHERE model_name = 'default-model') <> 0 OR
     (SELECT display_order FROM public.vendors WHERE name = 'default-vendor') <> 0 THEN
    RAISE EXCEPTION 'display-order defaults mismatch';
  END IF;
END
$contract$;
SQL

docker exec "$container" createdb --username=migration_contract marketplace_order_concurrency
docker exec --interactive "$container" psql \
  --username=migration_contract --dbname=marketplace_order_concurrency \
  --set=ON_ERROR_STOP=1 <"$migration" >/dev/null
postgres_port=$(docker port "$container" 5432/tcp | sed -n '1s/.*://p')
if [ -z "$postgres_port" ]; then
  echo "failed to resolve PostgreSQL test port" >&2
  exit 1
fi
(
  cd "$script_dir/../.."
  MARKETPLACE_ORDER_POSTGRES_TEST_DSN="postgresql://migration_contract:migration_contract@127.0.0.1:$postgres_port/marketplace_order_concurrency?sslmode=disable" \
    go test ./model -run 'MarketplaceDisplayOrderPostgres' -count=1
)

echo "Marketplace display-order migration contract passed"
