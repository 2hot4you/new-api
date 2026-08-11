#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
migration="$script_dir/20260811_molii_grok_management_credentials.sql"

if [ ! -r "$migration" ]; then
  echo "missing migration: $migration" >&2
  exit 1
fi

postgres_image=${POSTGRES_TEST_IMAGE:-postgres:15-alpine@sha256:3d0f7584ed7d04e27fa050d6683a74746608faf21f202be78460d679cc56461f}
container="molii-grok-management-migration-test-$$"

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM

docker run --detach --rm \
  --name "$container" \
  --env POSTGRES_DB=migration_contract \
  --env POSTGRES_PASSWORD=migration_contract \
  --env POSTGRES_USER=migration_contract \
  "$postgres_image" >/dev/null

attempt=0
until docker exec "$container" psql \
  --username=migration_contract \
  --dbname=migration_contract \
  --tuples-only \
  --command='SELECT 1' >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 30 ]; then
    echo "PostgreSQL test container did not become ready" >&2
    docker logs "$container" >&2
    exit 1
  fi
  sleep 1
done

docker exec --interactive "$container" psql \
  --username=migration_contract \
  --dbname=migration_contract \
  --set=ON_ERROR_STOP=1 <<'SQL' >/dev/null
CREATE TABLE public.channels (id bigint PRIMARY KEY);
SQL

run_migration() {
  docker exec --interactive "$container" psql \
    --username=migration_contract \
    --dbname=migration_contract \
    --set=ON_ERROR_STOP=1 <"$migration" >/dev/null
}

run_migration
run_migration

docker exec --interactive "$container" psql \
  --username=migration_contract \
  --dbname=migration_contract \
  --set=ON_ERROR_STOP=1 <<'SQL'
DO $contract$
DECLARE
  differences text;
BEGIN
  WITH expected(column_name, data_type, is_nullable) AS (
    VALUES
      ('molii_grok_management_access_token', 'text', 'NO'),
      ('molii_grok_management_user_id', 'integer', 'NO')
  ),
  actual AS (
    SELECT
      column_name::text,
      data_type::text,
      is_nullable::text
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'channels'
      AND column_name LIKE 'molii_grok_management_%'
  ),
  difference AS (
    (SELECT * FROM expected EXCEPT SELECT * FROM actual)
    UNION ALL
    (SELECT * FROM actual EXCEPT SELECT * FROM expected)
  )
  SELECT string_agg(row(difference.*)::text, E'\n' ORDER BY column_name)
  INTO differences
  FROM difference;

  IF differences IS NOT NULL THEN
    RAISE EXCEPTION 'Molii Grok management credential column contract mismatch:%', E'\n' || differences;
  END IF;

  INSERT INTO public.channels (id) VALUES (1);
  IF EXISTS (
    SELECT 1
    FROM public.channels
    WHERE id = 1
      AND (
        molii_grok_management_access_token <> '' OR
        molii_grok_management_user_id <> 0
      )
  ) THEN
    RAISE EXCEPTION 'Molii Grok management credential defaults are invalid';
  END IF;
END
$contract$;
SQL

echo "Molii Grok management credential migration contract passed"
