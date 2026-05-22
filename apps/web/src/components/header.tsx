import * as React from "react";
import { Logo } from "./logo";
import { Button } from "./ui/button";
import { Kbd } from "./ui/kbd";
import { cn } from "@/lib/utils";
import { BASE } from "@/lib/base";

interface HeaderProps {
  currentPath: string;
}

export function Header({ currentPath }: HeaderProps) {
  const onDocs = currentPath.startsWith("/docs");
  return (
    <header className="sticky top-0 z-40 w-full border-b border-border bg-background/85 backdrop-blur supports-[backdrop-filter]:bg-background/70">
      <div className="mx-auto flex h-14 max-w-screen-2xl items-center gap-4 px-4 sm:px-6 lg:px-8">
        {/* Mobile sidebar trigger — populated only on docs pages, hidden on lg+ */}
        {onDocs && (
          <button
            type="button"
            data-mobile-nav-trigger
            className="inline-flex h-9 w-9 items-center justify-center rounded-md text-foreground hover:bg-muted lg:hidden"
            aria-label="Toggle navigation"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M4 6h16M4 12h16M4 18h16" strokeLinecap="round" />
            </svg>
          </button>
        )}

        {/* Brand */}
        <a href={BASE + "/"} className="flex items-center gap-2 font-semibold tracking-tight">
          <Logo className="text-primary" />
          <span className="text-[15px]">pg-flux</span>
        </a>

        {/* Primary nav */}
        <nav className="hidden items-center gap-6 text-sm md:flex">
          <a
            href={BASE + "/docs/quick-start.html"}
            className={cn(
              "transition-colors hover:text-foreground",
              onDocs ? "text-foreground font-medium" : "text-muted-foreground",
            )}
          >
            Docs
          </a>
          <a
            href={BASE + "/docs/cli-overview.html"}
            className="text-muted-foreground transition-colors hover:text-foreground"
          >
            CLI
          </a>
          <a
            href="https://github.com/nex-gen-tech/pg-flux"
            target="_blank"
            rel="noopener"
            className="text-muted-foreground transition-colors hover:text-foreground"
          >
            GitHub
          </a>
        </nav>

        {/* Right cluster */}
        <div className="ml-auto flex items-center gap-2">
          {/* Search trigger */}
          <button
            type="button"
            data-search-trigger
            className="inline-flex h-9 items-center gap-2 rounded-md border border-border bg-background px-3 text-sm text-muted-foreground transition-colors hover:bg-muted sm:min-w-[200px]"
            aria-label="Search documentation"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="11" cy="11" r="8" />
              <path d="m21 21-4.3-4.3" strokeLinecap="round" />
            </svg>
            <span className="hidden sm:inline">Search docs...</span>
            <span className="ml-auto hidden sm:inline">
              <Kbd>⌘K</Kbd>
            </span>
          </button>

          {/* Theme toggle */}
          <button
            type="button"
            data-theme-toggle
            className="inline-flex h-9 w-9 items-center justify-center rounded-md text-foreground hover:bg-muted"
            aria-label="Toggle theme"
          >
            <svg className="hidden dark:block" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="4" />
              <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
            </svg>
            <svg className="dark:hidden" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
            </svg>
          </button>

          {/* GitHub icon (visible md+) */}
          <Button variant="ghost" size="icon" asChild className="hidden md:inline-flex">
            <a href="https://github.com/nex-gen-tech/pg-flux" target="_blank" rel="noopener" aria-label="GitHub">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.11.79-.25.79-.56 0-.27-.01-1.18-.02-2.13-3.2.7-3.87-1.36-3.87-1.36-.52-1.32-1.27-1.68-1.27-1.68-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.02 1.75 2.68 1.24 3.34.95.1-.74.4-1.24.72-1.53-2.55-.29-5.23-1.28-5.23-5.68 0-1.26.45-2.28 1.18-3.09-.12-.29-.51-1.46.11-3.04 0 0 .96-.31 3.16 1.18.92-.26 1.91-.39 2.89-.39.98 0 1.97.13 2.89.39 2.2-1.49 3.16-1.18 3.16-1.18.63 1.58.23 2.75.11 3.04.74.81 1.18 1.83 1.18 3.09 0 4.41-2.68 5.39-5.24 5.67.41.36.78 1.06.78 2.14 0 1.55-.01 2.79-.01 3.17 0 .31.21.68.79.56C20.21 21.39 23.5 17.08 23.5 12 23.5 5.65 18.35.5 12 .5z" />
              </svg>
            </a>
          </Button>
        </div>
      </div>
    </header>
  );
}
