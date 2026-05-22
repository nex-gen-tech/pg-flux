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
import { BASE } from "@/lib/base";

export function Landing() {
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
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
    <section className="relative border-b border-border">
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
          <Badge variant="outline" className="mb-5 gap-1.5 border-border py-1 pl-2 pr-3 text-muted-foreground">
            <span className="inline-flex h-1.5 w-1.5 rounded-full bg-primary" />
            v0.1 · PostgreSQL 14 – 18
          </Badge>
          <h1 className="text-4xl font-semibold leading-[1.05] tracking-tight text-foreground sm:text-5xl lg:text-[64px]">
            Your Postgres schema and your app types,
            <span className="block text-primary">
              <span className="editorial-tight">finally</span> in the same place.
            </span>
          </h1>
          <p className="mt-6 max-w-xl text-lg leading-relaxed text-muted-foreground">
            Write your schema as plain SQL. pg-flux generates the migration,
            applies it safely, and emits Go + TypeScript types that match —
            every time. No DSL. No second source of truth. No more 3 a.m.
            "wait, what type IS the email column."
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Button asChild size="lg">
              <a href={BASE + "/docs/quick-start.html"}>
                Quick start
                <ArrowRight size={16} />
              </a>
            </Button>
            <Button asChild size="lg" variant="outline">
              <a href="https://github.com/nex-gen-tech/pg-flux" target="_blank" rel="noopener">
                View on GitHub
              </a>
            </Button>
          </div>
          <div className="mt-8 flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <Check size={14} className="text-primary" /> Go 1.25+
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Check size={14} className="text-primary" /> MIT licensed
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Check size={14} className="text-primary" /> Zero-config defaults
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
      "Write SQL once. pg-flux figures out the migration. No HCL, no DSL, no JSON config of your tables — just SQL, the way PostgreSQL meant it.",
  },
  {
    icon: ShieldCheck,
    title: "Refuses to break prod",
    body:
      "Mass-drops blocked. Type rewrites blocked. Drift between generate and apply caught by a baseline-hash check. You have to opt in to anything risky.",
  },
  {
    icon: GitMerge,
    title: "Adopts against existing DBs",
    body:
      "One command extracts your live schema into source files, round-trip clean. The dump → migrate generate loop produces zero pending statements. Verified in CI.",
  },
  {
    icon: Code2,
    title: "Generates the types",
    body:
      "Go structs and TypeScript interfaces for every catalog object with a row shape. Branded IDs, zod validators, ORM tags — opt in to what you need, ignore the rest.",
  },
  {
    icon: Boxes,
    title: "PostgreSQL 14 through 18",
    body:
      "Virtual generated columns, NULLS NOT DISTINCT, NOT ENFORCED, security_invoker views — all version-gated and emitted only when the target server supports them.",
  },
  {
    icon: Workflow,
    title: "Built for CI",
    body:
      "Three exit codes: drift detected, generated code stale, undeclared live objects. Wire a pipeline that fails on any of them and you'll catch issues before they reach production.",
  },
];

function Features() {
  return (
    <section className="border-b border-border">
      <div className="mx-auto max-w-screen-2xl px-4 py-20 sm:px-6 lg:px-8">
        <div className="max-w-2xl">
          <p className="editorial text-xl text-primary">
            What's in the box
          </p>
          <h2 className="mt-1 text-3xl font-semibold tracking-tight sm:text-4xl">
            Every piece that used to be a separate tool.
          </h2>
          <p className="mt-4 text-base text-muted-foreground">
            One CLI handles the lifecycle: schema in SQL, diffed against live,
            applied safely, types generated, drift caught. Stop bolting tools together.
          </p>
        </div>
        <div className="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {FEATURES.map(({ icon: Icon, title, body }) => (
            <Card key={title} className="border-border bg-card">
              <CardContent className="p-6">
                <div className="mb-4 inline-flex h-9 w-9 items-center justify-center rounded-md bg-secondary text-primary">
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
    <section className="border-b border-border bg-muted/40">
      <div className="mx-auto max-w-screen-2xl px-4 py-20 sm:px-6 lg:px-8">
        <div className="max-w-2xl">
          <p className="editorial text-xl text-primary">
            How it works
          </p>
          <h2 className="mt-1 text-3xl font-semibold tracking-tight sm:text-4xl">
            The everyday workflow.
          </h2>
          <p className="mt-4 text-base text-muted-foreground">
            Four commands. Edit, generate, apply, regenerate. Each step is
            observable and idempotent.
          </p>
        </div>
        <ol className="mt-12 grid gap-6 lg:grid-cols-2">
          {STEPS.map((step, i) => (
            <li
              key={step.title}
              className="flex gap-5 rounded-xl border border-border bg-card p-6"
            >
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-border bg-background font-mono text-sm font-medium text-primary">
                {i + 1}
              </div>
              <div className="min-w-0 flex-1">
                <h3 className="mb-3 text-base font-medium tracking-tight">
                  {step.title}
                </h3>
                <pre className="overflow-x-auto rounded-md border border-code-border bg-code-bg px-3.5 py-3 font-mono text-[13px] leading-6 text-foreground">
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
    <section className="border-b border-border">
      <div className="mx-auto max-w-screen-2xl px-4 py-20 sm:px-6 lg:px-8">
        <div className="max-w-2xl">
          <p className="editorial text-xl text-primary">
            How it compares
          </p>
          <h2 className="mt-1 text-3xl font-semibold tracking-tight sm:text-4xl">
            One tool. No second source of truth.
          </h2>
          <p className="mt-4 text-base text-muted-foreground">
            Most teams bolt together a migration tool, a dump utility, and a
            codegen. pg-flux replaces all three with one model.
          </p>
        </div>
        <div className="mt-10 overflow-hidden rounded-xl border border-border">
          <table className="w-full text-sm">
            <thead className="bg-muted text-left text-muted-foreground">
              <tr>
                <th className="px-5 py-3 font-medium">Capability</th>
                <th className="px-5 py-3 font-medium text-primary">pg-flux</th>
                <th className="px-5 py-3 font-medium">Typical alternative</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr
                  key={r.label}
                  className={i === rows.length - 1 ? "" : "border-b border-border"}
                >
                  <td className="px-5 py-3 font-medium">{r.label}</td>
                  <td className="px-5 py-3 text-primary">{r.pgflux}</td>
                  <td className="px-5 py-3 text-muted-foreground">{r.others}</td>
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
    <section className="border-b border-border">
      <div className="mx-auto max-w-screen-2xl px-4 py-20 sm:px-6 lg:px-8">
        <div className="mx-auto max-w-3xl rounded-2xl border border-border bg-card p-8 text-center sm:p-12">
          <Terminal className="mx-auto mb-5 text-primary" size={28} />
          <h2 className="text-2xl font-semibold tracking-tight sm:text-3xl">
            Install and try it in two commands.
          </h2>
          <p className="mt-3 text-muted-foreground">
            Go install, then point at any local Postgres.
          </p>
          <div className="mt-6 inline-flex flex-col gap-2 rounded-lg border border-code-border bg-code-bg px-5 py-3 text-left font-mono text-sm">
            <code className="text-foreground">
              <span className="text-primary">$</span> go install
              github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest
            </code>
            <code className="text-foreground">
              <span className="text-primary">$</span> pg-flux init
            </code>
          </div>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Button asChild>
              <a href={BASE + "/docs/quick-start.html"}>
                Read the quick start
                <ArrowRight size={16} />
              </a>
            </Button>
            <Button variant="outline" asChild>
              <a href={BASE + "/docs/codegen.html"}>Codegen docs</a>
            </Button>
          </div>
        </div>
      </div>
    </section>
  );
}
