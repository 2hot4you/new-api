\set ON_ERROR_STOP on
-- Only temporary setup precedes the physically read-only persistent-table read.
SET statement_timeout='15s';
CREATE TEMP TABLE pg_temp.original_options (key text PRIMARY KEY,value text NOT NULL);
\copy pg_temp.original_options FROM '/repair/options.csv' WITH (FORMAT csv,HEADER true)
BEGIN READ ONLY;
SELECT NOT EXISTS (
 (SELECT * FROM pg_temp.original_options EXCEPT SELECT key,value FROM public.options WHERE key IN ('billing_setting.billing_expr','billing_setting.billing_mode'))
 UNION ALL
 (SELECT key,value FROM public.options WHERE key IN ('billing_setting.billing_expr','billing_setting.billing_mode') EXCEPT SELECT * FROM pg_temp.original_options)
);
ROLLBACK;
