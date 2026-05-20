/**
 * Landing page — single index.html showcasing pg-flux's value proposition,
 * commands, and a CTA into the docs. No client-side state.
 */
import type { Page } from "./types";
import { renderHeader, renderFooter } from "./layout";

interface LandingInput {
  pages: Page[];
}

export function renderLanding(_: LandingInput): string {
  return /* html */ `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>pg-flux — declarative PostgreSQL migrations + codegen</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="description" content="Declarative PostgreSQL migrations with safe apply, drift detection, schema dump, and end-to-end Go + TypeScript codegen.">
<link rel="stylesheet" href="/style.css">
</head>
<body class="landing">
${renderHeader()}
<main>
  <section class="hero">
    <div class="hero-text">
      <h1>One source of truth for your <span class="grad">Postgres schema</span> AND your app types.</h1>
      <p class="lede">
        pg-flux manages your PostgreSQL schema declaratively, generates safe migrations,
        catches drift, and emits idiomatic Go &amp; TypeScript types — keeping your code
        in lock-step with the database after every change.
      </p>
      <div class="cta">
        <a class="btn primary" href="/docs/quick-start.html">Quick start →</a>
        <a class="btn ghost" href="https://github.com/nexg/pg-flux">View on GitHub</a>
      </div>
      <p class="hero-meta">PG 14 / 15 / 16 / 17 / 18 · MIT licensed · Go 1.25+</p>
    </div>
    <pre class="hero-code"><code>$ pg-flux init
$ pg-flux migrate generate --label add_users
$ pg-flux migrate apply
$ pg-flux gen --lang go,ts --validators=zod</code></pre>
  </section>

  <section class="features">
    <h2>What it does</h2>
    <div class="feature-grid">
      <div class="feature">
        <h3>Declarative schema</h3>
        <p>Write SQL once in <code>schema/</code>. pg-flux diffs against the live DB and emits a minimal migration. No DSL, no second source of truth.</p>
      </div>
      <div class="feature">
        <h3>Safe apply</h3>
        <p>Mass-drop guard, drift detection between generate &amp; apply, advisory locking, transaction wrapping, NOT VALID + VALIDATE for FK/CHECK on big tables. Hazards surface as blockers in CI.</p>
      </div>
      <div class="feature">
        <h3>Dump · verify · pull</h3>
        <p>Adopt against an existing DB in one command. <code>verify</code> exits non-zero when something exists in the DB but not source. <code>pull</code> quarantines manual hotfixes for review.</p>
      </div>
      <div class="feature">
        <h3>Bidirectional codegen</h3>
        <p>Generate Go structs + TS interfaces for every table, enum, composite, domain, view, function, and procedure. Branded IDs, zod schemas, ORM tags (gorm/sqlx/bun), camelCase, optional, the works.</p>
      </div>
      <div class="feature">
        <h3>PG 14–18 coverage</h3>
        <p>NULLS NOT DISTINCT, virtual generated columns, named NOT NULL ... NOT VALID, NOT ENFORCED, security_invoker views — version-gated and emitted only when the target server supports them.</p>
      </div>
      <div class="feature">
        <h3>CI-friendly</h3>
        <p><code>verify --strict</code>, <code>gen --check</code>, <code>drift</code>. Build a CI pipeline that fails on undeclared objects, stale codegen, or out-of-band production drift.</p>
      </div>
    </div>
  </section>

  <section class="workflow">
    <h2>The workflow</h2>
    <ol class="steps">
      <li>
        <h3>Write SQL in <code>schema/</code></h3>
        <pre><code class="lang-sql">CREATE TABLE users (
  id bigint PRIMARY KEY,
  email text NOT NULL,
  role user_role NOT NULL DEFAULT 'member'
);</code></pre>
      </li>
      <li>
        <h3>Generate a migration</h3>
        <pre><code>$ pg-flux migrate generate --label add_role
Generated: migrations/20260520_add_role.sql (3 statements)</code></pre>
      </li>
      <li>
        <h3>Apply safely</h3>
        <pre><code>$ pg-flux migrate apply
apply 20260520_add_role.sql ... ok</code></pre>
      </li>
      <li>
        <h3>Regenerate types</h3>
        <pre><code>$ pg-flux gen
[go] wrote 2 files
[ts] wrote 4 files</code></pre>
      </li>
    </ol>
  </section>

  <section class="installation">
    <h2>Install</h2>
    <pre><code>$ go install github.com/nexg/pg-flux/cmd/pg-flux@latest</code></pre>
    <p>Or grab a release binary from <a href="https://github.com/nexg/pg-flux/releases">GitHub Releases</a>.</p>
    <p class="next-step">Next: <a href="/docs/quick-start.html">5-minute quick start →</a></p>
  </section>
</main>
${renderFooter()}
</body>
</html>`;
}
