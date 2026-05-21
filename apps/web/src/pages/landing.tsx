import * as React from "react";
import {
  ArrowRight,
  Database,
  ShieldCheck,
  GitMerge,
  Code2,
  Boxes,
  Workflow,
  Check,
  Terminal,
} from "lucide-react";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardTitle } from "@/components/ui/card";
import { TerminalHero } from "@/components/terminal-hero";

export function Landing() {
  return (
    <div className="flex min-h-screen flex-col bg-[--color-background] text-[--color-foreground]">
      <Header currentPath="/" />
      <main className="flex-1">
        <Hero />
        <Features />
        <Workflow_ />
        <Comparison />
        <CTA />
      </main>
      <Footer />
    </div>
  );
}

/* ------------------------------ Hero ------------------------------ */

function Hero() {
  return (
    <section className="relative border-b border-[--color-border]">
      {/* Subtle dotted background pattern — no gradient, just a few dots */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-[0.4] dark:opacity-[0.25]"
        style={{
          backgroundImage:
            "radial-gradient(circle, var(--color-border) 1px, transparent 1px)",
          backgroundSize: "24px 24px",
          maskImage:
            "radial-gradient(ellipse 80% 60% at 50% 30%, black 30%, transparent 80%)",
        }}
      />
      <div className="relative mx-auto grid max-w-screen-2xl gap-14 px-4 py-20 sm:px-6 sm:py-24 lg:grid-cols-[1.05fr_1fr] lg:gap-16 lg:px-8 lg:py-28">
        <div className="flex flex-col items-start">
          <Badge variant="outline" className="mb-5 gap-1.5 border-[--color-border] py-1 pl-2 pr-3 text-[--color-muted-foreground]">
            <span className="inline-flex h-1.5 w-1.5 rounded-full bg-[--color-primary]" />
            v0.1 · PostgreSQL 14 – 18
          </Badge>
          <h1 className="text-4xl font-semibold leading-[1.1] tracking-tight text-[--color-foreground] sm:text-5xl lg:text-6xl">
            One source of truth for your Postgres schema
            <span className="block text-[--color-primary]">and your app types.</span>
          </h1>
          <p className="mt-6 max-w-xl text-lg leading-relaxed text-[--color-muted-foreground]">
            Declarative migrations, safe apply, drift detection, schema dump, plus
            end-to-end Go &amp; TypeScript codegen. Your schema and your app stay in
            lock-step after every change.
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Button asChild size="lg">
              <a href="/docs/quick-start.html">
                Quick start
                <ArrowRight size={16} />
              </a>
            </Button>
            <Button asChild size="lg" variant="outline">
              <a href="https://github.com/nexg/pg-flux" target="_blank" rel="noopener">
                View on GitHub
              </a>
            </Button>
          </div>
          <div className="mt-8 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-[--color-muted-foreground]">
            <span className="inline-flex items-center gap-1.5">
              <Check size={14} className="text-[--color-primary]" /> Go 1.25+
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Check size={14} className="text-[--color-primary]" /> MIT licensed
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Check size={14} className="text-[--color-primary]" /> Zero-config defaults
            </span>
          </div>
        </div>

        {/* Animated terminal */}
        <TerminalHero />
      </div>
    </section>
  );
}

/* ---------------------------- Features ---------------------------- */

const FEATURES = [
  {
    icon: Database,
    title: "Declarative schema",
    body:
      "Write SQL once in schema/. pg-flux diffs against the live DB and emits a minimal migration. No DSL, no second source of truth.",
  },
  {
    icon: ShieldCheck,
    title: "Safe apply",
    body:
      "Mass-drop guard, drift detection between generate and apply, advisory locking, NOT VALID + VALIDATE auto-rewrites for FK & CHECK on large tables.",
  },
  {
    icon: GitMerge,
    title: "Bidirectional sync",
    body:
      "dump · verify · pull. Adopt against an existing DB in one command. Catch out-of-band changes before they reach production source.",
  },
  {
    icon: Code2,
    title: "Bidirectional codegen",
    body:
      "Go structs + TS interfaces for every table, enum, composite, domain, view, function, and procedure. Branded IDs, zod schemas, ORM tags.",
  },
  {
    icon: Boxes,
    title: "PG 14 – 18 coverage",
    body:
      "NULLS NOT DISTINCT, virtual generated columns, named NOT NULL NOT VALID, NOT ENFORCED, security_invoker views. Version-gated, fail-loud.",
  },
  {
    icon: Workflow,
    title: "CI-friendly",
    body:
      "verify --strict, gen --check, drift. Wire a pipeline that fails on stale generated code, undeclared objects, or production drift.",
  },
];

function Features() {
  return (
    <section className="border-b border-[--color-border]">
      <div className="mx-auto max-w-screen-2xl px-4 py-20 sm:px-6 lg:px-8">
        <div className="max-w-2xl">
          <p className="text-sm font-medium uppercase tracking-wider text-[--color-primary]">
            What it does
          </p>
          <h2 className="mt-2 text-3xl font-semibold tracking-tight sm:text-4xl">
            Everything pg-flux ships, in one tool.
          </h2>
          <p className="mt-4 text-base text-[--color-muted-foreground]">
            Migrations, drift detection, schema dump, and codegen — same model,
            one source of truth, every workflow.
          </p>
        </div>
        <div className="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {FEATURES.map(({ icon: Icon, title, body }) => (
            <Card key={title} className="border-[--color-border] bg-[--color-card]">
              <CardContent className="p-6">
                <div className="mb-4 inline-flex h-9 w-9 items-center justify-center rounded-md bg-[--color-secondary] text-[--color-primary]">
                  <Icon size={18} />
                </div>
                <CardTitle className="mb-2 text-base">{title}</CardTitle>
                <CardDescription className="text-[14px] leading-relaxed">
                  {body}
                </CardDescription>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  );
}

/* ---------------------------- Workflow ---------------------------- */

const STEPS = [
  {
    title: "Write SQL in schema/",
    code: `CREATE TABLE users (
  id    bigint PRIMARY KEY,
  email text NOT NULL,
  role  user_role NOT NULL DEFAULT 'member'
);`,
    lang: "sql",
  },
  {
    title: "Generate a migration",
    code: `$ pg-flux migrate generate --label add_role
Generated: migrations/20260520_add_role.sql (3 statements)`,
    lang: "bash",
  },
  {
    title: "Apply safely",
    code: `$ pg-flux migrate apply
apply 20260520_add_role.sql ... ok`,
    lang: "bash",
  },
  {
    title: "Regenerate types",
    code: `$ pg-flux gen
[go] wrote 2 files (4 already up to date)
[ts] wrote 4 files (6 already up to date)`,
    lang: "bash",
  },
];

function Workflow_() {
  return (
    <section className="border-b border-[--color-border] bg-[--color-muted]/40">
      <div className="mx-auto max-w-screen-2xl px-4 py-20 sm:px-6 lg:px-8">
        <div className="max-w-2xl">
          <p className="text-sm font-medium uppercase tracking-wider text-[--color-primary]">
            How it works
          </p>
          <h2 className="mt-2 text-3xl font-semibold tracking-tight sm:text-4xl">
            The everyday workflow.
          </h2>
          <p className="mt-4 text-base text-[--color-muted-foreground]">
            Four commands. Edit, generate, apply, regenerate. Each step is
            observable and idempotent.
          </p>
        </div>
        <ol className="mt-12 grid gap-6 lg:grid-cols-2">
          {STEPS.map((step, i) => (
            <li
              key={step.title}
              className="flex gap-5 rounded-xl border border-[--color-border] bg-[--color-card] p-6"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-[--color-border] bg-[--color-background] font-mono text-sm font-medium text-[--color-primary]">
                {i + 1}
              </div>
              <div className="min-w-0 flex-1">
                <h3 className="mb-3 text-base font-medium tracking-tight">
                  {step.title}
                </h3>
                <pre className="overflow-x-auto rounded-md border border-[--color-code-border] bg-[--color-code-bg] px-3.5 py-3 font-mono text-[13px] leading-6 text-[--color-foreground]">
                  <code>{step.code}</code>
                </pre>
              </div>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}

/* ---------------------------- Comparison ---------------------------- */

function Comparison() {
  const rows: { label: string; pgflux: string; others: string }[] = [
    { label: "Declarative source", pgflux: "✓ Native", others: "Usually no (sql up/down pairs)" },
    { label: "Bidirectional dump", pgflux: "✓ Round-trip clean", others: "pg_dump (noisy, not round-trip)" },
    { label: "Drift detection", pgflux: "✓ 3 layers", others: "Manual" },
    { label: "Go + TS codegen", pgflux: "✓ Built-in", others: "Separate tool (sqlc, prisma)" },
    { label: "PG 18 features", pgflux: "✓ Version-gated", others: "Often lagging" },
    { label: "CI gates", pgflux: "drift, verify, gen --check", others: "Custom scripts" },
  ];
  return (
    <section className="border-b border-[--color-border]">
      <div className="mx-auto max-w-screen-2xl px-4 py-20 sm:px-6 lg:px-8">
        <div className="max-w-2xl">
          <p className="text-sm font-medium uppercase tracking-wider text-[--color-primary]">
            How it compares
          </p>
          <h2 className="mt-2 text-3xl font-semibold tracking-tight sm:text-4xl">
            One tool. No second source of truth.
          </h2>
          <p className="mt-4 text-base text-[--color-muted-foreground]">
            Most teams bolt together a migration tool, a dump utility, and a
            codegen. pg-flux replaces all three with one model.
          </p>
        </div>
        <div className="mt-10 overflow-hidden rounded-xl border border-[--color-border]">
          <table className="w-full text-sm">
            <thead className="bg-[--color-muted] text-left text-[--color-muted-foreground]">
              <tr>
                <th className="px-5 py-3 font-medium">Capability</th>
                <th className="px-5 py-3 font-medium text-[--color-primary]">pg-flux</th>
                <th className="px-5 py-3 font-medium">Typical alternative</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr
                  key={r.label}
                  className={i === rows.length - 1 ? "" : "border-b border-[--color-border]"}
                >
                  <td className="px-5 py-3 font-medium">{r.label}</td>
                  <td className="px-5 py-3 text-[--color-primary]">{r.pgflux}</td>
                  <td className="px-5 py-3 text-[--color-muted-foreground]">{r.others}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

/* ---------------------------- CTA ---------------------------- */

function CTA() {
  return (
    <section className="border-b border-[--color-border]">
      <div className="mx-auto max-w-screen-2xl px-4 py-20 sm:px-6 lg:px-8">
        <div className="mx-auto max-w-3xl rounded-2xl border border-[--color-border] bg-[--color-card] p-8 text-center sm:p-12">
          <Terminal className="mx-auto mb-5 text-[--color-primary]" size={28} />
          <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
            Install and try it in two commands.
          </h2>
          <p className="mt-3 text-[--color-muted-foreground]">
            Go install, then point at any local Postgres.
          </p>
          <div className="mt-6 inline-flex flex-col gap-2 rounded-lg border border-[--color-code-border] bg-[--color-code-bg] px-5 py-3 text-left font-mono text-sm">
            <code className="text-[--color-foreground]">
              <span className="text-[--color-primary]">$</span> go install
              github.com/nexg/pg-flux/cmd/pg-flux@latest
            </code>
            <code className="text-[--color-foreground]">
              <span className="text-[--color-primary]">$</span> pg-flux init
            </code>
          </div>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Button asChild>
              <a href="/docs/quick-start.html">
                Read the quick start
                <ArrowRight size={16} />
              </a>
            </Button>
            <Button variant="outline" asChild>
              <a href="/docs/codegen.html">Codegen docs</a>
            </Button>
          </div>
        </div>
      </div>
    </section>
  );
}
