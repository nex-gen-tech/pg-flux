import * as React from "react";
import { Logo } from "./logo";
import { BASE } from "@/lib/base";

interface LinkGroup {
  title: string;
  links: { label: string; href: string; external?: boolean }[];
}

const GROUPS: LinkGroup[] = [
  {
    title: "Documentation",
    links: [
      { label: "Quick start", href: BASE + "/docs/quick-start.html" },
      { label: "Installation", href: BASE + "/docs/installation.html" },
      { label: "Migrations", href: BASE + "/docs/migrations.html" },
      { label: "Codegen", href: BASE + "/docs/codegen.html" },
      { label: "Hazards", href: BASE + "/docs/hazards.html" },
    ],
  },
  {
    title: "Reference",
    links: [
      { label: "CLI overview", href: BASE + "/docs/cli-overview.html" },
      { label: "Configuration", href: BASE + "/docs/configuration.html" },
      { label: "Drift recovery", href: BASE + "/docs/drift.html" },
      { label: "Dump · verify · pull", href: BASE + "/docs/dump.html" },
    ],
  },
  {
    title: "Community",
    links: [
      { label: "GitHub", href: "https://github.com/nex-gen-tech/pg-flux", external: true },
      { label: "Issues", href: "https://github.com/nex-gen-tech/pg-flux/issues", external: true },
      { label: "Discussions", href: "https://github.com/nex-gen-tech/pg-flux/discussions", external: true },
      { label: "Releases", href: "https://github.com/nex-gen-tech/pg-flux/releases", external: true },
    ],
  },
];

export function Footer() {
  return (
    <footer className="mt-24 border-t border-border bg-background">
      <div className="mx-auto max-w-screen-2xl px-4 py-12 sm:px-6 lg:px-8">
        <div className="grid gap-10 sm:grid-cols-2 lg:grid-cols-4 lg:gap-8">
          {/* Brand */}
          <div className="lg:col-span-1">
            <a href={BASE + "/"} className="inline-flex items-center gap-2 font-semibold tracking-tight">
              <Logo className="text-primary" size={20} />
              <span>pg-flux</span>
            </a>
            <p className="mt-3 max-w-xs text-sm leading-relaxed text-muted-foreground">
              Declarative PostgreSQL migrations and end-to-end type generation. One model
              for your schema and your application.
            </p>
            <div className="mt-5 flex items-center gap-3 text-muted-foreground">
              <a
                href="https://github.com/nex-gen-tech/pg-flux"
                target="_blank"
                rel="noopener"
                aria-label="GitHub"
                className="transition-colors hover:text-foreground"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.11.79-.25.79-.56 0-.27-.01-1.18-.02-2.13-3.2.7-3.87-1.36-3.87-1.36-.52-1.32-1.27-1.68-1.27-1.68-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.02 1.75 2.68 1.24 3.34.95.1-.74.4-1.24.72-1.53-2.55-.29-5.23-1.28-5.23-5.68 0-1.26.45-2.28 1.18-3.09-.12-.29-.51-1.46.11-3.04 0 0 .96-.31 3.16 1.18.92-.26 1.91-.39 2.89-.39.98 0 1.97.13 2.89.39 2.2-1.49 3.16-1.18 3.16-1.18.63 1.58.23 2.75.11 3.04.74.81 1.18 1.83 1.18 3.09 0 4.41-2.68 5.39-5.24 5.67.41.36.78 1.06.78 2.14 0 1.55-.01 2.79-.01 3.17 0 .31.21.68.79.56C20.21 21.39 23.5 17.08 23.5 12 23.5 5.65 18.35.5 12 .5z" />
                </svg>
              </a>
            </div>
          </div>

          {/* Link groups */}
          {GROUPS.map((g) => (
            <div key={g.title}>
              <h4 className="mb-3 text-xs font-semibold uppercase tracking-wider text-foreground">
                {g.title}
              </h4>
              <ul className="space-y-2 text-sm">
                {g.links.map((l) => (
                  <li key={l.href}>
                    <a
                      href={l.href}
                      target={l.external ? "_blank" : undefined}
                      rel={l.external ? "noopener" : undefined}
                      className="text-muted-foreground transition-colors hover:text-foreground"
                    >
                      {l.label}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        {/* Bottom bar */}
        <div className="mt-12 flex flex-col gap-3 border-t border-border pt-6 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <p>
            © {new Date().getFullYear()} pg-flux contributors. Released under the{" "}
            <a
              href="https://github.com/nex-gen-tech/pg-flux/blob/main/LICENSE"
              className="hover:text-foreground"
              target="_blank"
              rel="noopener"
            >
              MIT License
            </a>
            .
          </p>
          <div className="flex items-center gap-4">
            <span className="inline-flex items-center gap-1.5">
              <span className="inline-block h-1.5 w-1.5 rounded-full bg-[hsl(140_60%_50%)]" />
              PostgreSQL 14 – 18
            </span>
            <span>v0.1.3</span>
          </div>
        </div>
      </div>
    </footer>
  );
}
