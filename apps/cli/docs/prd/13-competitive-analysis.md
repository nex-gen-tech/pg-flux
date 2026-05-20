# 13 — Competitive Analysis

**Document:** Market Landscape & Competitive Positioning
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## Overview

The database schema migration space has several established tools. This document provides a technical comparison of each tool's approach, strengths, and gaps — with particular focus on why pg-flux fills a niche that existing tools do not.

---

## 13.1 Tool Categories

Schema migration tools fall into two categories:

| Category | Approach | Examples |
|----------|----------|---------|
| **Imperative** | Developer writes ordered, versioned migration files (e.g., `V001__add_users.sql`). Tool tracks which files have been applied. | Flyway, Liquibase, golang-migrate |
| **Declarative** | Developer writes the desired end-state schema. Tool calculates the diff and generates the migration. | Atlas, pg-schema-diff, pg-flux |

pg-flux is declarative, which is the higher-value category for teams that want GitOps-style schema management.

---

## 13.2 Flyway

**Origin:** JVM-based, created by Axel Fontaine (now Red Gate).

### Architecture
Flyway tracks applied migrations in a `flyway_schema_history` table. Developers write versioned `.sql` files (or Java migrations). Flyway executes them in order.

### Strengths
- Mature ecosystem (10+ years)
- Strong community support
- Supports 28 databases including PostgreSQL, MySQL, Oracle, SQL Server
- Enterprise edition has Undo migrations feature

### Weaknesses
| Weakness | Impact |
|----------|--------|
| JVM runtime required | Large deployment overhead (JRE/JDK); not suitable for Go microservices without extra tooling |
| No declarative diffing | Developers must write every migration manually; drift goes undetected |
| No PG18 awareness | No awareness of PG18 catalog changes, temporal constraints, uuidv7 |
| No zero-downtime built-in | No automatic index concurrency, NOT VALID patterns |
| No rename safety | Drop+add is the only way to rename a column |
| Large binary (JVM) | Docker images must include JRE; significant size overhead |

### Target User
Teams with a DBA culture, existing SQL expertise, and polyglot database environments.

---

## 13.3 Liquibase

**Origin:** JVM-based, Nathan Voxland (now Liquibase Inc.).

### Architecture
Liquibase uses an XML/YAML/JSON/SQL "changelog" format to define changes. It tracks applied changesets and can generate rollback scripts.

### Strengths
- Cross-database support (similar to Flyway)
- Declarative changeset format
- Enterprise: policy-driven change governance, quality checks

### Weaknesses
| Weakness | Impact |
|----------|--------|
| JVM runtime required | Same as Flyway |
| XML/YAML change format | Verbose; not pure SQL; loses PostgreSQL-specific syntax benefits |
| No declarative schema diffing | Must still write every changeset |
| No PG18 awareness | No PG18 catalog support |
| Complex configuration | High learning curve for the changeset format |

### Target User
Enterprise teams requiring audit trails and cross-database support.

---

## 13.4 golang-migrate

**Origin:** Open source Go library / CLI.

### Architecture
Simple imperative migrator: applies `NNN_description.up.sql` and `NNN_description.down.sql` files in order. No schema diffing.

### Strengths
- Simple and lightweight
- Written in Go; trivial to embed in Go applications
- Supports multiple database drivers

### Weaknesses
| Weakness | Impact |
|----------|--------|
| Purely imperative | Developer manually writes every migration |
| No drift detection | Schema drift between migrations and live DB is invisible |
| No hazard detection | Unsafe DDL goes through unchecked |
| No rename safety | Manual DROP+ADD |

### Target User
Go microservices teams who want a simple migration runner with no extras.

---

## 13.5 Atlas (ariga.io)

**Origin:** Ariga, Inc. — open source (MPL-2.0) with enterprise offering.

### Architecture
Atlas is declarative. Developers write schema in Atlas HCL format (or use `--dev-url` to import from an existing schema) and Atlas calculates the diff.

### Strengths
- Declarative, similar to pg-flux's approach
- Multi-database support (PG, MySQL, SQLite, etc.)
- Atlas Schema Language (HCL) provides IDE support
- Cloud-hosted migration registry (Atlas Cloud)
- CI/CD integration

### Weaknesses
| Weakness | Impact |
|----------|--------|
| HCL format, not SQL | Teams must learn a new DSL; existing SQL cannot be used directly |
| No PG18 awareness | Uses `information_schema`; limited PG18 catalog support |
| No rename detection | Atlas treats renames as drop+add; rename detection is an unsolved problem |
| Generic across databases | PG18-specific features (uuidv7, temporal constraints, virtual generated columns) are not first-class |
| No pg_query_go integration | Uses its own schema parsing, which may not perfectly parse all PostgreSQL syntax |
| Limited hazard rewriting | Some unsafe DDL is auto-rewritten but coverage is incomplete |

### Target User
Teams using multiple database engines who want a single declarative tool.

---

## 13.6 Stripe `pg-schema-diff`

**GitHub:** `github.com/stripe/pg-schema-diff`
**Language:** Go
**License:** MIT

This is the most technically similar competitor to pg-flux. A detailed analysis follows.

### Architecture
pg-schema-diff connects to two PostgreSQL databases (current and a "new" schema applied to a shadow DB) and diffs the actual PostgreSQL state via catalog queries.

Key patterns:
- Uses `CREATE DATABASE ... TEMPLATE template0` to create a shadow DB
- Applies desired schema to shadow DB, then diffs current vs. shadow
- Executes concurrent index builds with automatic `CONCURRENTLY` addition
- Supports `NOT VALID` constraint patterns
- Has a "plan errors" system for blocking dangerous operations

### Strengths
| Strength | Notes |
|----------|-------|
| Written in Go | Same ecosystem as pg-flux |
| Zero-downtime patterns | Concurrent index builds, NOT VALID FKs/CHECKs, advisory warnings |
| No JVM | Lightweight single binary |
| Shadow DB approach | Robust diffing via actual PostgreSQL state |
| MIT license | Commercial-friendly |

### Critical Weaknesses vs. pg-flux

| Weakness | pg-flux Advantage |
|----------|-----------------|
| **No rename support** | pg-schema-diff explicitly documents: "Renaming objects is not supported. A rename will be treated as a drop and add." This causes DATA_LOSS for column renames. pg-flux solves this with hint-based rename resolution. |
| **PostgreSQL 14–17 only** | pg-schema-diff does not support PG18. pg-flux is purpose-built for PG18 with all catalog breaking changes handled. |
| **No PG18 catalog queries** | pg-schema-diff uses `pg_attribute.attcacheoff` (removed in PG18), causing runtime errors on PG18 databases. |
| **No `uuidv7()` awareness** | pg-schema-diff does not understand PG18-specific functions and syntax. |
| **No RLS policy diffing** | pg-schema-diff does not inspect or diff Row-Level Security policies. |
| **Shadow DB approach limitations** | Requires `CREATEDB` privilege always; pg-flux uses shadow DB only for validation, with catalog-based diffing as the primary path (lower privilege requirement for basic use). |
| **No AI integration** | No structured JSON output designed for AI agent consumption. |
| **No hint system** | No mechanism for annotating migration intent in source files. |

### Feature Matrix

| Feature | pg-schema-diff | pg-flux |
|---------|---------------|---------|
| Declarative schema management | ✅ | ✅ |
| Written in Go | ✅ | ✅ |
| Zero-downtime index creation | ✅ | ✅ |
| NOT VALID constraint patterns | ✅ | ✅ |
| PostgreSQL 17 support | ✅ | ✅ |
| **PostgreSQL 18 support** | ❌ | ✅ |
| **Rename detection** | ❌ | ✅ |
| **RLS policy diffing** | ❌ | ✅ |
| **`uuidv7()` awareness** | ❌ | ✅ |
| **Temporal constraints** | ❌ | ✅ |
| **Named NOT NULL (PG18)** | ❌ | ✅ |
| **AI integration JSON output** | ❌ | ✅ |
| **Hint comment system** | ❌ | ✅ |
| **Shadow DB validation** | ✅ | ✅ |
| Multi-database support | ❌ | ❌ |

---

## 13.7 Summary Comparison Table

| Tool | Approach | Language | PG18 | Renames | RLS | Hazards | Binary |
|------|----------|----------|------|---------|-----|---------|--------|
| Flyway | Imperative | JVM | ❌ | ❌ | ❌ | ❌ | JRE needed |
| Liquibase | Imperative (XML) | JVM | ❌ | ❌ | ❌ | ❌ | JRE needed |
| golang-migrate | Imperative | Go | ❌ | ❌ | ❌ | ❌ | Small |
| Atlas | Declarative (HCL) | Go | ❌ | ❌ | ⚠️ | ⚠️ | Medium |
| pg-schema-diff | Declarative (SQL) | Go | ❌ | ❌ | ❌ | ✅ | Small |
| **pg-flux** | **Declarative (SQL)** | **Go** | **✅** | **✅** | **✅** | **✅** | **Small** |

---

## 13.8 Market Positioning

pg-flux occupies the top-right quadrant of a two-axis positioning:

```
                    PostgreSQL 18 Native
                           ↑
                           │
              pg-flux      │
                           │
─────────────────────────────────────────── → Declarative
                           │       pg-schema-diff
                           │
    Flyway / Liquibase     │       Atlas
                           │
                    Imperative
```

**Primary differentiation:** The only declarative migration tool that is purpose-built for PostgreSQL 18, with first-class support for PG18 catalog changes, new syntax, and zero-downtime migration patterns specific to PG18 features.

**Secondary differentiation:** The only tool with hint-based rename resolution, eliminating the most common cause of data loss in declarative schema migration.
