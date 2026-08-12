#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
migration="$script_dir/20260812_default_token.sql"
postgres_image=${POSTGRES_TEST_IMAGE:-postgres:15-alpine@sha256:3d0f7584ed7d04e27fa050d6683a74746608faf21f202be78460d679cc56461f}
container="default-token-migration-test-$$"

cleanup() {
  docker rm -f "$container" >/dev/null 2>&1 || true
}
trap cleanup EXIT HUP INT TERM

docker run --detach --rm --name "$container" \
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
CREATE TABLE public.tokens (
  id bigserial PRIMARY KEY,
  user_id bigint NOT NULL,
  deleted_at timestamptz
);
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
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = 'public' AND table_name = 'tokens'
      AND column_name = 'is_default' AND data_type = 'boolean'
      AND column_default = 'false'
      AND is_nullable = 'NO'
  ) THEN
    RAISE EXCEPTION 'tokens.is_default contract mismatch';
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes
    WHERE schemaname = 'public' AND tablename = 'tokens'
      AND indexname = 'ux_tokens_one_live_default_per_user'
  ) THEN
    RAISE EXCEPTION 'default token unique index is missing';
  END IF;
END
$contract$;

INSERT INTO public.tokens (user_id, is_default) VALUES (1, true);
DO $contract$
BEGIN
  BEGIN
    INSERT INTO public.tokens (user_id, is_default) VALUES (1, true);
    RAISE EXCEPTION 'expected duplicate default token to fail';
  EXCEPTION WHEN unique_violation THEN
    NULL;
  END;
END
$contract$;
UPDATE public.tokens SET deleted_at = now() WHERE user_id = 1;
INSERT INTO public.tokens (user_id, is_default) VALUES (1, true);
SQL

echo "Default token migration contract passed"
