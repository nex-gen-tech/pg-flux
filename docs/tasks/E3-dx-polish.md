# Epic 3 — Developer Experience Polish

**Priority:** P1 — friction reduction; does not block CI story but directly affects interactive workflow.
**Spec ref:** [Spec 1 §Open Gaps G8, G9](../spec/1/spec.md)

---

## E3-T1 — pg-flux migrate rehash (accept edited migration)

**Summary:** Any manual edit to a generated migration file (e.g., removing a broken statement, adding a comment) changes the file's embedded hash. Every subsequent `migrate apply` then requires `--force-after-drift`, which blanket-skips all drift checks. There is no targeted way to say "I reviewed and accepted this edit; update its hash."

**Proposal:** Add `pg-flux migrate rehash [<file>]` (or `accept`) that re-computes and writes the hash for a specific migration file after a user edit, without touching other files or bypassing apply-time checks.

**Acceptance criteria:**
- `pg-flux migrate rehash migrations/20260521_xxx.sql` updates the `pg-flux-baseline-hash` line in that file.
- Subsequent `migrate apply` no longer requires `--force-after-drift` for that file.
- Rehashing does not skip the content-vs-DB diff at apply time — it only accepts the file's new content as canonical.
- Help text explains when to use this vs `--force-after-drift`.

**Dependencies:** None.

---

## E3-T2 — init skips or warns on existing schema files

**Summary:** `pg-flux init` writes a sample `schema/users.sql`. If the user runs `init` in a directory that already has schema files (e.g., when reinitializing or following the README), the sample silently overwrites existing work.

**Acceptance criteria:**
- `init` detects existing files in `schema_dir` and skips writing sample files.
- OR `init` prompts before overwriting (with non-tty fallback: skip, not overwrite).
- The `.pg-flux.yml` file is still written (or updated) regardless.
- Behavior is documented in `init --help`.

**Dependencies:** None.
