\set ON_ERROR_STOP on
-- Offline DEV-only maintenance. Caller stops the app and durably backs up first.
BEGIN;
SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='15s';
SET LOCAL idle_in_transaction_session_timeout='30s';
SET LOCAL search_path=public,pg_temp;
DO $$ BEGIN
 PERFORM 1 FROM marketplace_order_locks WHERE name='marketplace_display_order' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'missing marketplace anchor'; END IF;
 PERFORM 1 FROM catalog_sync_states WHERE id=1 AND publication_state='ready'
  AND pending_operation_id='' AND revision=runtime_revision FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'catalog is not fully published'; END IF;
END $$;
LOCK TABLE models,channels,abilities IN SHARE MODE;
LOCK TABLE options IN SHARE ROW EXCLUSIVE MODE;
CREATE TEMP TABLE pg_temp.approved_options (key text PRIMARY KEY,value text NOT NULL) ON COMMIT DROP;
\copy pg_temp.approved_options FROM '/repair/options.csv' WITH (FORMAT csv,HEADER true)
CREATE TEMP TABLE pg_temp.untouched_options ON COMMIT DROP AS
 SELECT key,value FROM options WHERE key NOT IN ('billing_setting.billing_expr','billing_setting.billing_mode');
DO $$ BEGIN
 IF (SELECT count(*) FROM pg_temp.approved_options)<>2 OR EXISTS (
  SELECT 1 FROM pg_temp.approved_options WHERE key NOT IN ('billing_setting.billing_expr','billing_setting.billing_mode')
 ) THEN RAISE EXCEPTION 'backup must contain exactly the two approved rows'; END IF;
 IF (SELECT count(*) FROM options WHERE key IN ('billing_setting.billing_expr','billing_setting.billing_mode'))<>2
 THEN RAISE EXCEPTION 'both current options are required'; END IF;
 IF EXISTS (SELECT 1 FROM pg_temp.approved_options WHERE json_typeof(value::json)<>'object')
 THEN RAISE EXCEPTION 'backup values must be JSON objects'; END IF;
 IF EXISTS (SELECT 1 FROM pg_temp.approved_options o CROSS JOIN LATERAL json_each(o.value::json) e
  GROUP BY o.key,e.key HAVING count(*)<>1)
 THEN RAISE EXCEPTION 'duplicate JSON keys are unsafe'; END IF;
 IF EXISTS (SELECT 1 FROM pg_temp.approved_options o CROSS JOIN LATERAL json_each(o.value::json) e WHERE json_typeof(e.value)<>'string')
 OR EXISTS (SELECT 1 FROM pg_temp.approved_options WHERE NOT (value::jsonb ? 'claude-opus-4-6'))
 THEN RAISE EXCEPTION 'expected string-valued maps with the approved leaf'; END IF;
 IF EXISTS (SELECT 1 FROM tasks WHERE status IN ('NOT_START','SUBMITTED','QUEUED','IN_PROGRESS','UNKNOWN'))
 THEN RAISE EXCEPTION 'active task found'; END IF;
END $$;
\if :restore
DO $$ DECLARE original_count int; post_count int; BEGIN
 SELECT count(*) INTO original_count FROM options o JOIN pg_temp.approved_options b USING(key) WHERE o.value=b.value;
 SELECT count(*) INTO post_count FROM options o JOIN pg_temp.approved_options b USING(key)
  WHERE o.value=(b.value::jsonb-'claude-opus-4-6')::text;
 IF original_count=2 THEN RETURN; END IF;
 IF post_count<>2 THEN RAISE EXCEPTION 'restore CAS failed; inspect without replay'; END IF;
 UPDATE options o SET value=b.value FROM pg_temp.approved_options b WHERE o.key=b.key
  AND o.value=(b.value::jsonb-'claude-opus-4-6')::text;
 IF NOT FOUND THEN RAISE EXCEPTION 'restore did not update expected rows'; END IF;
 IF EXISTS (SELECT 1 FROM options o JOIN pg_temp.approved_options b USING(key) WHERE o.value<>b.value)
 THEN RAISE EXCEPTION 'restore postcondition failed'; END IF;
END $$;
\else
DO $$ DECLARE affected int; orphans int; BEGIN
 IF EXISTS (SELECT 1 FROM options o JOIN pg_temp.approved_options b USING(key) WHERE o.value<>b.value)
 THEN RAISE EXCEPTION 'backup/current raw value mismatch'; END IF;
 IF EXISTS (SELECT 1 FROM models WHERE model_name='claude-opus-4-6' AND deleted_at IS NULL)
 OR EXISTS (SELECT 1 FROM abilities WHERE model='claude-opus-4-6' AND enabled)
 OR EXISTS (SELECT 1 FROM channels WHERE status=1 AND 'claude-opus-4-6'=ANY(string_to_array(models,',')))
 THEN RAISE EXCEPTION 'legacy model still referenced; stop for review'; END IF;
 WITH price_leaves AS (
  SELECT o.key,jsonb_object_keys(o.value::jsonb) AS model_name FROM options o
  WHERE o.key IN ('AudioCompletionRatio','AudioRatio','CacheRatio','CompletionRatio','CreateCacheRatio','ImageRatio','ModelPrice','ModelRatio','billing_setting.billing_expr','billing_setting.billing_mode')
 ) SELECT count(*) INTO orphans FROM price_leaves p WHERE NOT EXISTS (
  SELECT 1 FROM models m WHERE m.model_name=p.model_name AND m.name_rule=0
   AND m.billing_currency IN ('USD','CNY') AND m.deleted_at IS NULL
 );
 IF orphans<>2 THEN RAISE EXCEPTION 'orphan scope changed; stop for review'; END IF;
 UPDATE options o SET value=(b.value::jsonb-'claude-opus-4-6')::text FROM pg_temp.approved_options b
  WHERE o.key=b.key AND o.value=b.value;
 GET DIAGNOSTICS affected=ROW_COUNT;
 IF affected<>2 THEN RAISE EXCEPTION 'expected exactly two changed rows'; END IF;
 IF EXISTS (SELECT 1 FROM options o JOIN pg_temp.approved_options b USING(key)
  WHERE o.value::jsonb<>(b.value::jsonb-'claude-opus-4-6'))
 THEN RAISE EXCEPTION 'cleanup postcondition failed'; END IF;
END $$;
\endif
DO $$ BEGIN
 IF EXISTS ((SELECT * FROM pg_temp.untouched_options EXCEPT SELECT key,value FROM options WHERE key NOT IN ('billing_setting.billing_expr','billing_setting.billing_mode'))
 UNION ALL (SELECT key,value FROM options WHERE key NOT IN ('billing_setting.billing_expr','billing_setting.billing_mode') EXCEPT SELECT * FROM pg_temp.untouched_options))
 THEN RAISE EXCEPTION 'an unrelated option changed'; END IF;
END $$;
COMMIT;
SELECT 'approved two-row maintenance verified';
