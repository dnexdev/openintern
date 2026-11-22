#!/usr/bin/env bash
# Export all OpenIntern state from Postgres to CSV and JSONL in one consistent
# snapshot.
#
#   DATABASE_URL=... scripts/export-state.sh <out-dir>
#
# Needs only psql (pg_dump optional). Every table in the public and drizzle
# schemas is copied inside one REPEATABLE READ, READ ONLY transaction, with
# timestamps rendered by Postgres in UTC (microseconds kept, no JS Date
# rounding). Writes row counts, consistency checks and SHA256SUMS.
# Read-only and takes seconds of compute: run it first when Neon is readable.
set -euo pipefail

out="${1:?usage: export-state.sh <out-dir>}"
: "${DATABASE_URL:?DATABASE_URL is required}"
mkdir -p "$out"
cd "$out"

export PGOPTIONS="-c TimeZone=UTC -c DateStyle=ISO,YMD -c extra_float_digits=3"
psql_q() { psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -qAt "$@"; }

psql_q -c "select version()" > server-version.txt
tables=$(psql_q -c "select quote_ident(schemaname) || '.' || quote_ident(tablename)
  from pg_tables where schemaname in ('public', 'drizzle') order by 1")
[ -n "$tables" ] || { echo "no tables found" >&2; exit 1; }

{
  echo "BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;"
  for t in $tables; do
    f="${t//\"/}.csv"
    echo "\\copy (select * from $t order by 1) to '$f' with (format csv, header)"
    # The same rows as JSON (row_to_json), the input for flat-file state.
    echo "\\pset format unaligned"
    echo "\\pset tuples_only on"
    echo "\\o ${f%.csv}.jsonl"
    echo "select row_to_json(x)::text from $t x order by 1;"
    echo "\\o"
    echo "select '$t', count(*) from $t;"
  done
  # Fingerprints must equal sha256(company_id|source|external_job_id).
  echo "select 'fingerprint_mismatch', count(*) from jobs
    where fingerprint <> encode(sha256(convert_to(company_id::text || '|' || source || '|' || external_job_id, 'UTF8')), 'hex');"
  # An ATS remap (company ats changed after jobs were created) changes future fingerprints.
  echo "select 'source_ne_company_ats', count(*) from jobs j join companies c on c.id = j.company_id
    where j.source <> c.ats::text;"
  echo "select 'jobs_active', count(*) from jobs where is_active;"
  echo "select 'snapshot_at', now();"
  echo "COMMIT;"
} > export.sql

psql_q -F $'\t' -f export.sql > counts.tsv
cat counts.tsv

if command -v pg_dump >/dev/null; then
  if ! pg_dump "$DATABASE_URL" -Fc --no-owner --no-acl -f neon.dump 2> pg_dump.err; then
    echo "warning: pg_dump failed (often a client/server version mismatch); CSVs are complete:" >&2
    cat pg_dump.err >&2
    rm -f neon.dump
  fi
fi

sha256sum -- *.csv *.jsonl counts.tsv server-version.txt $( [ -f neon.dump ] && echo neon.dump ) > SHA256SUMS
echo "export complete: $(pwd)"
