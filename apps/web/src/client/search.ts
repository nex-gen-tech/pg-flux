/**
 * ⌘K search modal. Loads search-index.json once, runs a token-match scorer
 * on every keystroke. Keyboard nav (↑ ↓ Enter Esc), click-outside-to-dismiss.
 */
interface IndexEntry {
  title: string;
  href: string;
  group: string;
  body: string;
}

let indexPromise: Promise<IndexEntry[]> | null = null;
function getIndex(): Promise<IndexEntry[]> {
  if (!indexPromise) {
    indexPromise = fetch("/search-index.json")
      .then((r) => r.json() as Promise<IndexEntry[]>)
      .catch(() => []);
  }
  return indexPromise;
}

function score(entry: IndexEntry, tokens: string[]): number {
  const title = entry.title.toLowerCase();
  const body = entry.body.toLowerCase();
  let s = 0;
  for (const t of tokens) {
    if (title === t) s += 50;
    else if (title.includes(t)) s += 12;
    if (body.includes(t)) s += 1;
  }
  return s;
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]!));
}

class SearchModal {
  private overlay: HTMLDivElement;
  private input: HTMLInputElement;
  private results: HTMLDivElement;
  private hits: IndexEntry[] = [];
  private active = -1;

  constructor() {
    this.overlay = document.createElement("div");
    this.overlay.className =
      "fixed inset-0 z-[60] hidden items-start justify-center bg-black/50 backdrop-blur-sm pt-[10vh] px-4";
    this.overlay.setAttribute("role", "dialog");
    this.overlay.setAttribute("aria-modal", "true");
    this.overlay.innerHTML = `
      <div class="w-full max-w-xl overflow-hidden rounded-xl border border-border bg-card shadow-2xl">
        <div class="flex items-center gap-2 border-b border-border px-4">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-muted-foreground"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3" stroke-linecap="round"/></svg>
          <input
            type="search"
            data-search-input
            placeholder="Search docs..."
            class="h-12 w-full border-0 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
            autocomplete="off" autocorrect="off" autocapitalize="off" spellcheck="false"
          />
          <kbd class="hidden h-5 select-none items-center rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground sm:inline-flex">Esc</kbd>
        </div>
        <div data-search-results class="max-h-[60vh] overflow-y-auto p-2"></div>
      </div>
    `;
    document.body.appendChild(this.overlay);
    this.input = this.overlay.querySelector<HTMLInputElement>("[data-search-input]")!;
    this.results = this.overlay.querySelector<HTMLDivElement>("[data-search-results]")!;

    this.input.addEventListener("input", () => this.update());
    this.input.addEventListener("keydown", (e) => this.keydown(e));
    this.overlay.addEventListener("mousedown", (e) => {
      if (e.target === this.overlay) this.close();
    });
  }

  open() {
    this.overlay.classList.remove("hidden");
    this.overlay.classList.add("flex");
    this.input.value = "";
    this.results.innerHTML = `<div class="px-3 py-6 text-center text-sm text-muted-foreground">Start typing to search…</div>`;
    setTimeout(() => this.input.focus(), 0);
  }

  close() {
    this.overlay.classList.add("hidden");
    this.overlay.classList.remove("flex");
  }

  isOpen() {
    return !this.overlay.classList.contains("hidden");
  }

  private async update() {
    const q = this.input.value.trim().toLowerCase();
    if (!q) {
      this.hits = [];
      this.active = -1;
      this.results.innerHTML = `<div class="px-3 py-6 text-center text-sm text-muted-foreground">Start typing to search…</div>`;
      return;
    }
    const idx = await getIndex();
    const tokens = q.split(/\s+/);
    this.hits = idx
      .map((p) => ({ p, s: score(p, tokens) }))
      .filter((x) => x.s > 0)
      .sort((a, b) => b.s - a.s)
      .slice(0, 8)
      .map((x) => x.p);
    this.active = this.hits.length > 0 ? 0 : -1;
    this.render();
  }

  private render() {
    if (this.hits.length === 0) {
      this.results.innerHTML = `<div class="px-3 py-6 text-center text-sm text-muted-foreground">No matches.</div>`;
      return;
    }
    this.results.innerHTML = this.hits
      .map(
        (h, i) => `
        <a href="${h.href}" data-search-hit="${i}"
           class="flex items-start justify-between gap-3 rounded-md px-3 py-2.5 text-sm transition-colors ${
             i === this.active
               ? "bg-secondary text-foreground"
               : "text-foreground hover:bg-muted"
           }">
          <div class="min-w-0">
            <div class="truncate font-medium">${escapeHtml(h.title)}</div>
            <div class="truncate text-xs text-muted-foreground">${escapeHtml(h.group)}</div>
          </div>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="mt-1 shrink-0 text-muted-foreground"><path d="M7 17 17 7M7 7h10v10" stroke-linecap="round"/></svg>
        </a>
      `,
      )
      .join("");
    for (const a of this.results.querySelectorAll<HTMLAnchorElement>("[data-search-hit]")) {
      a.addEventListener("mouseenter", () => {
        this.active = Number(a.getAttribute("data-search-hit"));
        this.render();
      });
    }
  }

  private keydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.preventDefault();
      this.close();
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      if (this.hits.length === 0) return;
      this.active = (this.active + 1) % this.hits.length;
      this.render();
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      if (this.hits.length === 0) return;
      this.active = (this.active - 1 + this.hits.length) % this.hits.length;
      this.render();
    } else if (e.key === "Enter") {
      if (this.active >= 0 && this.hits[this.active]) {
        e.preventDefault();
        window.location.href = this.hits[this.active]!.href;
      }
    }
  }
}

let modal: SearchModal | null = null;

function init() {
  modal = new SearchModal();
  for (const btn of document.querySelectorAll<HTMLButtonElement>("[data-search-trigger]")) {
    btn.addEventListener("click", () => modal!.open());
  }
  document.addEventListener("keydown", (e) => {
    if ((e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      if (modal!.isOpen()) modal!.close();
      else modal!.open();
    }
    if (e.key === "/" && document.activeElement?.tagName !== "INPUT" && document.activeElement?.tagName !== "TEXTAREA") {
      e.preventDefault();
      modal!.open();
    }
  });
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
else init();
