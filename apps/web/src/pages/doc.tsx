import * as React from "react";
import { Header } from "@/components/header";
import { Sidebar } from "@/components/sidebar";
import { Toc } from "@/components/toc";
import { Footer } from "@/components/footer";
import { Badge } from "@/components/ui/badge";
import type { Page } from "@/lib/content";

interface DocPageProps {
  page: Page;
  pages: Page[];
}

export function DocPage({ page, pages }: DocPageProps) {
  const idx = pages.findIndex((p) => p.slug === page.slug);
  const prev = idx > 0 ? pages[idx - 1] : null;
  const next = idx < pages.length - 1 ? pages[idx + 1] : null;

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <Header currentPath={page.href} />

      {/* Mobile nav drawer — populated by sidebar.tsx markup; visibility toggled by client JS */}
      <div data-mobile-nav className="fixed inset-0 z-50 hidden">
        <div data-mobile-nav-backdrop className="absolute inset-0 bg-black/40" />
        <aside className="absolute left-0 top-0 h-full w-72 max-w-[80vw] overflow-y-auto border-r border-border bg-background px-4 py-6">
          <Sidebar pages={pages} currentSlug={page.slug} />
        </aside>
      </div>

      <div className="mx-auto flex w-full max-w-screen-2xl flex-1 gap-8 px-4 sm:px-6 lg:px-8">
        {/* Sidebar — desktop only, sticky */}
        <aside className="sticky top-14 hidden h-[calc(100vh-3.5rem)] w-60 shrink-0 overflow-y-auto py-8 lg:block">
          <Sidebar pages={pages} currentSlug={page.slug} />
        </aside>

        {/* Content */}
        <main className="min-w-0 flex-1 py-8 lg:max-w-3xl xl:max-w-none">
          <article className="mx-auto max-w-[720px] xl:mx-0">
            <header className="mb-8">
              <p className="editorial mb-2 text-lg text-primary">{page.group}</p>
              <h1 className="text-4xl font-semibold tracking-tight text-foreground">
                {page.title}
              </h1>
              {page.description && (
                <p className="mt-3 text-base leading-relaxed text-muted-foreground">{page.description}</p>
              )}
            </header>

            <div className="prose" dangerouslySetInnerHTML={{ __html: page.html }} />

            {(prev || next) && (
              <nav className="mt-16 grid gap-3 border-t border-border pt-6 sm:grid-cols-2">
                {prev ? (
                  <a
                    href={prev.href}
                    className="group rounded-lg border border-border p-4 transition-colors hover:bg-muted"
                  >
                    <div className="text-xs text-muted-foreground">← Previous</div>
                    <div className="mt-1 font-medium text-foreground group-hover:text-primary">
                      {prev.title}
                    </div>
                  </a>
                ) : (
                  <span />
                )}
                {next ? (
                  <a
                    href={next.href}
                    className="group rounded-lg border border-border p-4 text-right transition-colors hover:bg-muted"
                  >
                    <div className="text-xs text-muted-foreground">Next →</div>
                    <div className="mt-1 font-medium text-foreground group-hover:text-primary">
                      {next.title}
                    </div>
                  </a>
                ) : (
                  <span />
                )}
              </nav>
            )}
          </article>
        </main>

        {/* TOC — xl only, sticky right */}
        <aside className="sticky top-14 hidden h-[calc(100vh-3.5rem)] w-56 shrink-0 overflow-y-auto py-8 xl:block">
          <Toc items={page.toc} />
        </aside>
      </div>

      <Footer />
    </div>
  );
}
