import * as React from "react";
import { Rocket, GitMerge, Workflow, Code2, Settings, BookOpen, FileCode2 } from "lucide-react";
import { cn } from "@/lib/utils";
import type { Page } from "@/lib/content";

interface SidebarProps {
  pages: Page[];
  currentSlug: string;
  className?: string;
}

// Icon per group label. Falls back to BookOpen for unknown groups.
const GROUP_ICONS: Record<string, React.ComponentType<{ size?: number; className?: string }>> = {
  "Getting started": Rocket,
  Migrations: GitMerge,
  "Dump & sync": Workflow,
  Codegen: Code2,
  Configuration: Settings,
  Reference: FileCode2,
};

/** Docs sidebar — groups pages, icon per group, strong active state. */
export function Sidebar({ pages, currentSlug, className }: SidebarProps) {
  const groups = groupPages(pages);
  return (
    <nav className={cn("text-sm", className)} aria-label="Documentation">
      {Object.entries(groups).map(([group, items]) => {
        const Icon = GROUP_ICONS[group] ?? BookOpen;
        return (
          <div key={group} className="mb-6">
            <h4 className="mb-2 flex items-center gap-2 px-2 text-[11px] font-semibold uppercase tracking-wider text-[--color-muted-foreground]">
              <Icon size={12} className="text-[--color-primary]" />
              <span>{group}</span>
            </h4>
            <ul className="space-y-px pl-2">
              {items.map((p) => {
                const active = p.slug === currentSlug;
                return (
                  <li key={p.slug}>
                    <a
                      href={p.href}
                      className={cn(
                        "block rounded-md px-2.5 py-1.5 text-[13.5px] transition-colors",
                        active
                          ? "nav-item-active"
                          : "text-[--color-muted-foreground] hover:bg-[--color-muted] hover:text-[--color-foreground]",
                      )}
                    >
                      {p.title}
                    </a>
                  </li>
                );
              })}
            </ul>
          </div>
        );
      })}
    </nav>
  );
}

function groupPages(pages: Page[]) {
  const out: Record<string, Page[]> = {};
  for (const p of pages) (out[p.group] ||= []).push(p);
  const order = [
    "Getting started",
    "Migrations",
    "Dump & sync",
    "Codegen",
    "Configuration",
    "Reference",
  ];
  const sorted: Record<string, Page[]> = {};
  for (const g of order) if (out[g]) sorted[g] = out[g];
  for (const g of Object.keys(out)) if (!sorted[g]) sorted[g] = out[g];
  return sorted;
}
