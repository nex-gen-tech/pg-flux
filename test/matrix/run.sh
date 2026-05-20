#!/usr/bin/env bash
# Comprehensive mutation matrix: 18 sequential schema versions × 5 PG versions.
# For each step: generate → apply → drift; record any failure to /tmp/full_matrix_report.txt.

BIN=/Users/ramanandakairi/nexg/pg-flux/pg-flux
REPORT=/tmp/full_matrix_report.txt
:>"$REPORT"

declare -a STEPS=(01_baseline 02_add_column 03_rename_column 04_type_change 05_add_check 06_function_meta 07_alter_policy 08_seq_change 09_enum_add 10_index_add 11_comment_owner 12_grants 13_set_not_null 14_drop_column 15_view_body 16_trigger_redef 17_default_priv 18_revoke_grant 19_partition_add 20_identity_col 21_generated_col 22_unlogged_reloptions 23_event_trigger 24_composite_type 25_composite_alter 26_alter_owner)

run_pg () {
  local label="$1"
  local port="$2"
  local dsn="postgres://pgflux:pgflux@localhost:${port}/pgflux_fm?sslmode=disable"
  local migdir=/tmp/full_mat_${label}

  printf "\n=== PG %s ===\n" "$label" | tee -a "$REPORT"
  PGPASSWORD=pgflux psql -h localhost -p "$port" -U pgflux -d postgres -c "DROP DATABASE IF EXISTS pgflux_fm;" >/dev/null 2>&1
  PGPASSWORD=pgflux psql -h localhost -p "$port" -U pgflux -d postgres -c "CREATE DATABASE pgflux_fm;" >/dev/null 2>&1
  PGPASSWORD=pgflux psql -h localhost -p "$port" -U pgflux -d pgflux_fm -c "DO \$\$BEGIN CREATE ROLE app_reader NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END\$\$; DO \$\$BEGIN CREATE ROLE app_writer NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END\$\$; DO \$\$BEGIN CREATE ROLE app_owner; EXCEPTION WHEN duplicate_object THEN NULL; END\$\$;" >/dev/null 2>&1
  rm -rf "$migdir" && mkdir -p "$migdir"

  for step in "${STEPS[@]}"; do
    local schema_dir=/tmp/full_mat_schema_$step
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
  PGPASSWORD=pgflux psql -h localhost -p "$port" -U pgflux -d postgres -c "DROP DATABASE pgflux_fm;" >/dev/null 2>&1
}

run_pg 14 5441
run_pg 15 5442
run_pg 16 5443
run_pg 17 5440
run_pg 18 5444

printf "\n=== SUMMARY ===\n"
echo "Total OK:        $(grep -c ': OK$' "$REPORT")"
echo "Total APPLY-FAIL: $(grep -c ': APPLY-FAIL' "$REPORT")"
echo "Total GEN-FAIL:  $(grep -c ': GEN-FAIL' "$REPORT")"
echo "Total DRIFT:     $(grep -c ': DRIFT:' "$REPORT")"
