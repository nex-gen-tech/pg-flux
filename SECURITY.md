# Security policy

pg-flux executes DDL against production databases. The trust boundary is meaningful, so we take security reports seriously.

## What counts as a security issue

| Issue type | Severity |
|---|---|
| Bypassing a hazard guard (mass-drop, baseline-hash drift, blocking hazards) without explicit opt-in | **Critical** |
| SQL injection through user-supplied identifiers (table names, column names, etc.) | **Critical** |
| Privilege escalation — pg-flux running with role X that gains access to objects role X cannot see | **High** |
| Reading credentials from disk / env in ways the user didn't authorize | **High** |
| Path traversal in schema-dir / migrations-dir handling | **High** |
| Crash / DoS on malformed input that wastes operator time but doesn't corrupt data | Medium |
| Misleading error messages that could lead an operator to apply the wrong DDL | Medium |

## Reporting

**Do not open a public GitHub issue.** That broadcasts the vulnerability before there's a fix.

Email the maintainers at the address listed on the project GitHub profile, or use GitHub's private vulnerability reporting from the Security tab. Include:

- A description of the vulnerability
- Steps to reproduce (or a proof-of-concept)
- pg-flux version (`pg-flux version`)
- PostgreSQL version (`SELECT version();`)
- Operating system

## What happens next

| Time | Step |
|---|---|
| Within 72 hours | Acknowledgement that the report was received |
| Within 7 days | Initial assessment + severity classification |
| Within 30 days | Fix in main + advisory drafted |
| Within 90 days | Coordinated disclosure (or sooner if a CVE warrants it) |

If we can't meet a window we'll tell you and adjust together.

## Disclosure

We use [GitHub Security Advisories](https://docs.github.com/en/code-security/security-advisories). After a fix is released, we publish the advisory with credit to the reporter (unless you prefer to remain anonymous).

## What we don't consider a vulnerability

- Hazards that pg-flux already refuses to apply by default (DROP TABLE, COLUMN TYPE CHANGE, mass-drop) — these are *features*. Operators can opt in with `--allow-hazards`.
- The user choosing to pass `--force-after-drift` and then experiencing drift consequences.
- The user manually editing applied migrations and getting a checksum mismatch.
- Connection-pool exhaustion when running with stupidly high parallelism.

If you're not sure whether your finding qualifies, report it anyway. We'd rather triage a non-issue than miss a real one.
