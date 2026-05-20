#!/usr/bin/env bash
# Comprehensive mutation matrix: 26 sequential schema versions × 5 PG versions.
# For each step: generate → apply → drift; record any failure to "$REPORT".
#
# Env overrides (defaults match the local pgflux-test-* Docker containers):
#   BIN           path to pg-flux binary (defaults to repo-root /pg-flux)
#   REPORT        path for the per-step log (defaults to /tmp/full_matrix_report.txt)
#   PGUSER/PGPASS the superuser to connect as
#   PG14_PORT … PG18_PORT  host port for each PG major
#   PGHOST        host name (defaults to localhost)

set -u

BIN=${BIN:-$(cd "$(dirname "$0")/../.." && pwd)/pg-flux}
REPORT=${REPORT:-/tmp/full_matrix_report.txt}
PGUSER=${PGUSER:-pgflux}
PGPASS=${PGPASS:-pgflux}
PGHOST=${PGHOST:-localhost}
PG14_PORT=${PG14_PORT:-5441}
PG15_PORT=${PG15_PORT:-5442}
PG16_PORT=${PG16_PORT:-5443}
PG17_PORT=${PG17_PORT:-5440}
PG18_PORT=${PG18_PORT:-5444}

:>"$REPORT"

declare -a STEPS=(01_baseline 02_add_column 03_rename_column 04_type_change 05_add_check 06_function_meta 07_alter_policy 08_seq_change 09_enum_add 10_index_add 11_comment_owner 12_grants 13_set_not_null 14_drop_column 15_view_body 16_trigger_redef 17_default_priv 18_revoke_grant 19_partition_add 20_identity_col 21_generated_col 22_unlogged_reloptions 23_event_trigger 24_composite_type 25_composite_alter 26_alter_owner)

run_pg () {
  local label="$1"
  local port="$2"
  local dsn="postgres://${PGUSER}:${PGPASS}@${PGHOST}:${port}/pgflux_fm?sslmode=disable"
  local migdir=/tmp/full_mat_${label}

  printf "\n=== PG %s ===\n" "$label" | tee -a "$REPORT"
  PGPASSWORD="$PGPASS" psql -h "$PGHOST" -p "$port" -U "$PGUSER" -d postgres -c "DROP DATABASE IF EXISTS pgflux_fm;" >/dev/null 2>&1
  PGPASSWORD="$PGPASS" psql -h "$PGHOST" -p "$port" -U "$PGUSER" -d postgres -c "CREATE DATABASE pgflux_fm;" >/dev/null 2>&1
  PGPASSWORD="$PGPASS" psql -h "$PGHOST" -p "$port" -U "$PGUSER" -d pgflux_fm -c "DO \$\$BEGIN CREATE ROLE app_reader NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END\$\$; DO \$\$BEGIN CREATE ROLE app_writer NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END\$\$; DO \$\$BEGIN CREATE ROLE app_owner; EXCEPTION WHEN duplicate_object THEN NULL; END\$\$;" >/dev/null 2>&1
  rm -rf "$migdir" && mkdir -p "$migdir"

  for step in "${STEPS[@]}"; do
    local schema_dir=/tmp/full_mat_schema_${label}_${step}
    rm -rf "$schema_dir" && mkdir -p "$schema_dir"
    cp "$(dirname "$0")/$step.sql" "$schema_dir/schema.sql"

    local gen_out
    gen_out=$("$BIN" migrate generate --label "$step" --db "$dsn" --schema "$schema_dir" --migrations-dir "$migdir" 2>&1)
    if echo "$gen_out" | grep -q "Error\|^pg-flux:.*requires"; then
      printf "  %s: GEN-FAIL: %s\n" "$step" "$(echo "$gen_out" | grep -E 'Error|requires' | head -1)" | tee -a "$REPORT"
      continue
    fi

    local apply_out
    apply_out=$("$BIN" migrate apply --db "$dsn" --schema "$schema_dir" --migrations-dir "$migdir" 2>&1)
    if echo "$apply_out" | grep -q "Error\|exec: ERROR\|^Error:"; then
      local err
      err=$(echo "$apply_out" | grep -E 'Error|exec: ERROR' | head -1)
      printf "  %s: APPLY-FAIL: %s\n" "$step" "$err" | tee -a "$REPORT"
      continue
    fi

    local drift_out
    drift_out=$("$BIN" drift --db "$dsn" --schema "$schema_dir" 2>&1 | grep -vE '^(Usage|  -|Global|$|Flags|pg-flux drift| -)')
    if echo "$drift_out" | grep -q "^No drift"; then
      printf "  %s: OK\n" "$step" | tee -a "$REPORT"
    else
      local d1
      d1=$(echo "$drift_out" | head -1)
      printf "  %s: DRIFT: %s\n" "$step" "$d1" | tee -a "$REPORT"
    fi
  done
  PGPASSWORD="$PGPASS" psql -h "$PGHOST" -p "$port" -U "$PGUSER" -d postgres -c "DROP DATABASE pgflux_fm;" >/dev/null 2>&1
}

run_pg 14 "$PG14_PORT"
run_pg 15 "$PG15_PORT"
run_pg 16 "$PG16_PORT"
run_pg 17 "$PG17_PORT"
run_pg 18 "$PG18_PORT"

printf "\n=== SUMMARY ===\n"
ok=$(grep -c ': OK$' "$REPORT")
apply=$(grep -c ': APPLY-FAIL' "$REPORT")
gen=$(grep -c ': GEN-FAIL' "$REPORT")
drift=$(grep -c ': DRIFT:' "$REPORT")
echo "Total OK:        $ok"
echo "Total APPLY-FAIL: $apply"
echo "Total GEN-FAIL:  $gen"
echo "Total DRIFT:     $drift"

# Exit non-zero on any failure so CI fails the job.
if [ "$apply" -gt 0 ] || [ "$gen" -gt 0 ] || [ "$drift" -gt 0 ]; then
  exit 1
fi
