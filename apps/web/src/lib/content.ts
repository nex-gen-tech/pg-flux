/**
 * Markdown → page model. Walks content/docs/**.md, parses frontmatter,
 * renders to HTML with marked + shiki, extracts a flat TOC for the right rail,
 * and post-processes the rendered HTML to add code-block headers, copy
 * buttons, and styled callouts.
 */
import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { Marked } from "marked";
import { BASE } from "./base";
import matter from "gray-matter";
import { createHighlighter, type Highlighter } from "shiki";

export interface Page {
  slug: string;
  href: string;
  title: string;
  group: string;
  order: number;
  description?: string;
  html: string;
  raw: string;
  toc: TocItem[];
}

export interface TocItem {
  id: string;
  text: string;
  depth: number;
}

const CONTENT_DIR = new URL("../../content/docs", import.meta.url).pathname;

async function findMarkdown(dir: string, prefix = ""): Promise<string[]> {
  const out: string[] = [];
  const entries = await readdir(dir, { withFileTypes: true });
  for (const e of entries) {
    const full = join(dir, e.name);
    const rel = prefix ? `${prefix}/${e.name}` : e.name;
    if (e.isDirectory()) out.push(...(await findMarkdown(full, rel)));
    else if (e.name.endsWith(".md")) out.push(rel);
  }
  return out;
}

function slugify(s: string): string {
  return s
    .toLowerCase()
    .replace(/<[^>]+>/g, "")
    .replace(/[^\w\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/^-+|-+$/g, "");
}

let cachedHighlighter: Promise<Highlighter> | null = null;
function getHighlighter() {
  if (!cachedHighlighter) {
    cachedHighlighter = createHighlighter({
      themes: ["github-light", "github-dark"],
      langs: ["bash", "shell", "sql", "go", "typescript", "tsx", "yaml", "json", "javascript", "text", "diff", "dockerfile"],
    });
  }
  return cachedHighlighter;
}

/**
 * Strip the leading H1 from markdown if it matches the frontmatter title.
 * The page layout already renders a styled header with badge + title +
 * description; the markdown body should be content only.
 */
function stripLeadingH1(body: string, title: string): string {
  const lines = body.split("\n");
  let i = 0;
  while (i < lines.length && lines[i].trim() === "") i++;
  if (i >= lines.length) return body;
  const m = lines[i].match(/^#\s+(.+)$/);
  if (m && m[1].trim().toLowerCase() === title.trim().toLowerCase()) {
    lines.splice(i, 1);
    while (i < lines.length && lines[i].trim() === "") i++;
    lines.splice(0, i);
    return lines.join("\n");
  }
  return body;
}

/**
 * Post-process the rendered HTML to:
 *  1. Wrap every <pre class="shiki"> in a code-block container with a
 *     language label (top-left) and a copy button (top-right).
 *  2. Convert GitHub-style admonition blockquotes ([!NOTE], [!WARNING], etc.)
 *     into styled callout components.
 */
function postProcessHtml(html: string): string {
  // ---- Code blocks: wrap with header + copy button ----
  // Match <pre> elements emitted by shiki (any class containing 'shiki').
  // Capture the FULL pre tag plus its inner code element so we can preserve
  // shiki's color spans inside.
  html = html.replace(
    /<pre((?:[^>]*?))>([\s\S]*?)<\/pre>/g,
    (full, preAttrs: string, inner: string) => {
      // Only target shiki-rendered blocks (skip raw <pre> we don't recognise).
      if (!/class=["'][^"']*shiki/.test(preAttrs)) return full;
      const langMatch = inner.match(/<code[^>]*data-language=["']([^"']+)["']/);
      const langAttr = langMatch ? langMatch[1] : extractLangFromCode(inner);
      const lang = langAttr || "text";
      const display = displayLangLabel(lang);
      // Encode the plain code text into a data attribute so client JS can copy
      // it without HTML-stripping. We extract it once at render time.
      const plain = decodeShikiHtml(inner);
      const dataCode = escapeAttr(plain);
      return `<div class="code-block" data-code-block>
  <div class="code-block-header">
    <span class="code-block-lang">${escapeHtml(display)}</span>
    <button type="button" class="code-block-copy" data-copy-code="${dataCode}" aria-label="Copy code">
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="code-block-copy-icon"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="code-block-check-icon hidden"><polyline points="20 6 9 17 4 12"/></svg>
      <span class="code-block-copy-label">Copy</span>
    </button>
  </div>
  <pre${preAttrs}>${inner}</pre>
</div>`;
    },
  );

  // ---- Callouts: blockquote starting with [!NOTE] / [!WARNING] / [!TIP] / [!INFO] ----
  html = html.replace(
    /<blockquote>\s*<p>\[!(NOTE|WARNING|TIP|INFO|IMPORTANT|CAUTION)\]\s*(?:<br ?\/?>)?\s*([\s\S]*?)<\/p>\s*([\s\S]*?)<\/blockquote>/gi,
    (_, kind: string, first: string, rest: string) => {
      const variant = kind.toLowerCase();
      const icon = calloutIcon(variant);
      return `<aside class="callout callout-${variant}" role="note">
  <div class="callout-icon">${icon}</div>
  <div class="callout-body">
    <p class="callout-title">${escapeHtml(calloutLabel(variant))}</p>
    <p>${first}</p>
    ${rest}
  </div>
</aside>`;
    },
  );

  return html;
}

function extractLangFromCode(inner: string): string {
  // shiki sometimes encodes lang only on the wrapper div, not the code tag.
  // Fall back to scanning for a `lang-<x>` class.
  const m = inner.match(/class=["'][^"']*(?:lang-|language-)([\w+-]+)/);
  return m ? m[1] : "";
}

function displayLangLabel(lang: string): string {
  const map: Record<string, string> = {
    ts: "TypeScript",
    tsx: "TypeScript",
    typescript: "TypeScript",
    js: "JavaScript",
    javascript: "JavaScript",
    bash: "Shell",
    shell: "Shell",
    sh: "Shell",
    sql: "SQL",
    yaml: "YAML",
    yml: "YAML",
    json: "JSON",
    go: "Go",
    text: "Text",
    diff: "Diff",
    dockerfile: "Dockerfile",
  };
  return map[lang.toLowerCase()] ?? lang.toUpperCase();
}

function calloutLabel(v: string): string {
  return v.charAt(0).toUpperCase() + v.slice(1).toLowerCase();
}

function calloutIcon(v: string): string {
  const icons: Record<string, string> = {
    note: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>',
    info: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>',
    tip: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18h6"/><path d="M10 22h4"/><path d="M15.09 14c.18-.98.65-1.74 1.41-2.5A4.65 4.65 0 0 0 18 8 6 6 0 0 0 6 8c0 1 .23 2.23 1.5 3.5A4.61 4.61 0 0 1 8.91 14"/></svg>',
    important: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 9v4"/><path d="M12 17h.01"/><path d="M21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/></svg>',
    warning: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 9v4"/><path d="M12 17h.01"/><path d="M21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/></svg>',
    caution: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m15 9-6 6"/><path d="m9 9 6 6"/></svg>',
  };
  return icons[v] ?? icons.note!;
}

/** Strip HTML tags from a shiki-rendered code chunk to get plain code text. */
function decodeShikiHtml(html: string): string {
  return html
    .replace(/<[^>]+>/g, "")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&nbsp;/g, " ");
}

function escapeAttr(s: string): string {
  return s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]!));
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]!));
}

export async function loadAllPages(): Promise<Page[]> {
  const highlighter = await getHighlighter();
  const files = await findMarkdown(CONTENT_DIR);
  const pages: Page[] = [];

  for (const rel of files) {
    const src = await readFile(join(CONTENT_DIR, rel), "utf8");
    const { data: meta, content: rawBody } = matter(src);
    const title = (meta.title as string) || deriveTitle(rawBody) || rel.replace(/\.md$/, "");
    const body = stripLeadingH1(rawBody, title);

    const toc: TocItem[] = [];
    const md = new Marked({
      gfm: true,
      breaks: false,
      renderer: {
        code({ text, lang }) {
          const safeLang = lang && highlighter.getLoadedLanguages().includes(lang as never)
            ? (lang as never)
            : "text";
          const inner = highlighter.codeToHtml(text, {
            lang: safeLang,
            themes: { light: "github-light", dark: "github-dark" },
            defaultColor: false,
          });
          // Re-emit with a data-language attribute so our post-processor knows the lang.
          return inner.replace(/<code/, `<code data-language="${safeLang}"`);
        },
        heading({ tokens, depth }) {
          const text = this.parser.parseInline(tokens);
          const id = slugify(text);
          if (depth >= 2 && depth <= 4) {
            toc.push({ id, text: text.replace(/<[^>]+>/g, ""), depth });
          }
          return `<h${depth} id="${id}"><a class="anchor" href="#${id}">#</a>${text}</h${depth}>`;
        },
      },
    });

    let html = md.parse(body) as string;
    html = postProcessHtml(html);

    const slug = "docs/" + rel.replace(/\.md$/, "").replace(/\\/g, "/");
    pages.push({
      slug,
      href: BASE + "/" + slug + ".html",
      title,
      group: (meta.group as string) || "Reference",
      order: meta.order ? Number(meta.order) : 99,
      description: meta.description as string | undefined,
      html,
      raw: rawBody,
      toc,
    });
  }
  return pages.sort((a, b) => a.order - b.order || a.title.localeCompare(b.title));
}

function deriveTitle(md: string): string {
  const m = md.match(/^#\s+(.+)$/m);
  return m ? m[1].trim() : "";
}
