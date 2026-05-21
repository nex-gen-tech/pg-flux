---
title: Dump / verify / pull
group: Reference
order: 4
description: Read-only commands for adoption and out-of-band change detection.
---

These three commands handle the bidirectional flow: live DB → source files.

## `pg-flux dump`

```bash
pg-flux dump [--output ./schema] [--layout per-kind|flat] [--force]
```

| Flag | Default | Description |
|---|---|---|
| `--output <dir>` | `./schema` | Destination |
| `--layout <kind>` | `per-kind` | `per-kind` (one file per object kind) or `flat` (single `schema.sql`) |
| `--force` | off | Overwrite a non-empty target directory |

The dump is **round-trip clean**: running `pg-flux migrate generate` immediately after a dump produces zero pending statements. That property is enforced by an integration test gate.

> [!TIP]
> Use `dump` to adopt pg-flux against an existing database. Run it once,
> commit the output, run `pg-flux migrate generate --label baseline` —
> you should see "0 statements".

---

## `pg-flux verify`

```bash
pg-flux verify [--strict]
```

Read-only asymmetric diff: lists every catalog object present in live but **not** declared in source. `--strict` exits 1 when any are found — pair with CI.

### Example

```bash
$ pg-flux verify --strict
verify: 2 undeclared live object(s):

  Tables (1):
    - public.hotfix_overrides

  Indexes (1):
    - public.idx_users_legacy

Run `pg-flux pull` to capture these into schema/_pulled/<ts>.sql for review.
```

Exit codes: `0` clean, `4` undeclared objects found.

---

## `pg-flux pull`

```bash
pg-flux pull [--dry-run=true] [--output ./schema/_pulled]
```

| Flag | Default | Description |
|---|---|---|
| `--dry-run` | `true` | Print the would-be SQL; don't touch disk |
| `--output <dir>` | `./schema/_pulled` | Quarantine directory |

Renders every live-only object via the same emitters `dump` uses, into a single timestamped quarantine file.

> [!IMPORTANT]
> `pull` never modifies your existing source files. The quarantine file is
> reviewed by hand; users move blocks into the regular source tree manually.

### Example

```bash
$ pg-flux pull --dry-run=false
Wrote 1 object(s) to ./schema/_pulled/20260520_103245_pulled.sql
```
