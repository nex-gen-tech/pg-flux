import * as React from "react";
import { cn } from "@/lib/utils";
import type { TocItem } from "@/lib/content";

interface TocProps {
  items: TocItem[];
  className?: string;
}

/** Right-rail table of contents — flat list of h2/h3 with depth indent. */
export function Toc({ items, className }: TocProps) {
  if (items.length === 0) return null;
  return (
    <nav className={cn("text-sm", className)} aria-label="On this page">
      <h4 className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
        On this page
      </h4>
      <ul className="space-y-1 border-l border-border">
        {items.map((item) => (
          <li key={item.id}>
            <a
              href={`#${item.id}`}
              data-toc-link
              data-toc-target={item.id}
              className={cn(
                "block border-l-2 border-transparent py-0.5 pl-3 -ml-px text-muted-foreground transition-colors hover:text-foreground",
                item.depth === 3 && "pl-5",
                item.depth === 4 && "pl-7",
              )}
            >
              {item.text}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
