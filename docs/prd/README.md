# pg-flux — Product Requirements Document Index

**Project:** Declarative PostgreSQL 18 Schema Migration Engine (`pg-flux`)
**Version:** 1.0
**Status:** Active Draft
**Last Updated:** April 2026

**PRD v2 (robustness roadmap):** **[PRD-v2-robustness.md](./PRD-v2-robustness.md)** — planned scope to close v1 gaps (loader completeness, types/partitions, execution, NFR). v1 documents describe current shipping intent; reconcile with implementation where they diverge.

---

## Document Structure

This PRD is organized into focused documents, each addressing a distinct product or engineering concern. Read them in order for full context, or jump to a specific section as needed.

| # | Document | Description |
|---|----------|-------------|
| — | **[README.md](./README.md)** | This index |
| v2 | **[PRD v2 — Robustness](./PRD-v2-robustness.md)** | Gap closure, phased milestones, FR2-01+, NFR2-* (planning) |
| 01 | **[Executive Summary](./01-executive-summary.md)** | Vision, problem statement, solution overview |
| 02 | **[Personas & User Stories](./02-personas-and-user-stories.md)** | Target users, use cases, acceptance criteria |
| 03 | **[Technical Architecture](./03-technical-architecture.md)** | System design, pipeline, module breakdown |
| 04 | **[Functional Requirements](./04-functional-requirements.md)** | All FR specs with acceptance criteria |
| 05 | **[PostgreSQL 18 Specifics](./05-postgres18-specific-features.md)** | PG18 features, catalog changes, optimizations |
| 06 | **[CLI Interface & UX](./06-cli-interface.md)** | Command design, flags, output formats |
| 07 | **[Non-Functional Requirements](./07-non-functional-requirements.md)** | Performance, portability, reliability benchmarks |
| 08 | **[Implementation Roadmap](./08-implementation-roadmap.md)** | Phased delivery plan with milestones |
| 09 | **[Risk Analysis](./09-risk-analysis.md)** | Technical and operational risks, mitigations |
| 10 | **[Testing Strategy](./10-testing-strategy.md)** | Unit, integration, shadow DB, chaos testing |
| 11 | **[Edge Cases & Known Limitations](./11-edge-cases-and-known-limitations.md)** | Boundary conditions, unsupported migrations |
| 12 | **[Security Considerations](./12-security-considerations.md)** | Threat model, OWASP mapping, hardening |
| 13 | **[Competitive Analysis](./13-competitive-analysis.md)** | Flyway, Liquibase, pg-schema-diff, Atlas comparison |

---

## Quick Reference: Core Concepts

### What is pg-flux?
A **declarative, state-based PostgreSQL 18 schema migration engine** written in Go. Rather than managing ordered `up`/`down` scripts, developers define their entire schema in `.sql` files. `pg-flux` computes the exact diff between the desired state and the live database, then generates a safe, sequenced, zero-downtime DDL execution plan.

### The Three Pillars
1. **Declarative** — Schema lives in version-controlled `.sql` files; no imperative scripts.
2. **Safe** — Every generated DDL statement passes through a Hazard Detection engine before execution.
3. **PG18-Native** — Purpose-built for PostgreSQL 18's new catalog structure, `uuidv7()`, async I/O, temporal constraints, and named `NOT NULL` constraints.

### Key Differentiators from Competitors

| Feature | pg-flux | pg-schema-diff (Stripe) | Atlas | Flyway/Liquibase |
|---------|---------|------------------------|-------|-----------------|
| Hint-based Rename Detection | ✅ | ❌ (drop+add) | Partial | ❌ |
| PostgreSQL 18 Native Support | ✅ | ❌ | Partial | ❌ |
| RLS Policy Diffing | ✅ | ❌ | ❌ | ❌ |
| PL/pgSQL Function Diffing | ✅ | ❌ | Partial | ❌ |
| Temporal Constraint Support | ✅ | ❌ | ❌ | ❌ |
| AI Integration Hooks (JSON output) | ✅ | ❌ | ❌ | ❌ |
| Zero-downtime Hazard Engine | ✅ | ✅ | Partial | ❌ |
| Single Static Binary | ✅ | ✅ | ✅ | ❌ (JVM) |

---

## Glossary

| Term | Definition |
|------|-----------|
| **Desired State** | The schema defined in `.sql` source files in the repository |
| **Current State** | The live schema extracted from PostgreSQL system catalogs |
| **AST** | Abstract Syntax Tree — the structured tree representation of parsed SQL |
| **Diff** | The calculated delta between Desired State and Current State |
| **Execution Plan** | The ordered list of DDL statements required to move from Current to Desired State |
| **Hazard** | An operation in an Execution Plan that risks data loss, table locking, or production instability |
| **DAG** | Directed Acyclic Graph — used for topological dependency ordering |
| **Shadow Database** | A temporary copy of the schema used for plan validation before live execution |
| **NOT VALID** | A PostgreSQL constraint option that defers validation to avoid full table scans |
| **CONCURRENTLY** | A PostgreSQL index build option that avoids Access Exclusive locks |
| **RLS** | Row-Level Security — per-row access control policies on PostgreSQL tables |
| **Hint Comment** | A structured SQL comment (e.g., `-- @renamed from=old_name`) used to provide diffing intent |
