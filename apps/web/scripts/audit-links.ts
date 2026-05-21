import { readdir, readFile, stat } from "node:fs/promises";
import { join, relative } from "node:path";

const DIST = "dist";

async function walk(dir: string): Promise<string[]> {
  const out: string[] = [];
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const p = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...(await walk(p)));
    else if (p.endsWith(".html")) out.push(p);
  }
  return out;
}

async function exists(p: string): Promise<boolean> {
  try {
    await stat(p);
    return true;
  } catch {
    return false;
  }
}

const html = await walk(DIST);
let total = 0;
const broken: { from: string; href: string }[] = [];

for (const f of html) {
  const text = await readFile(f, "utf8");
  for (const m of text.matchAll(/href="([^"]+)"/g)) {
    const href = m[1];
    total++;
    if (!href.startsWith("/")) continue;
    if (href.startsWith("//")) continue;
    const base = href.split("#")[0];
    if (!base || base === "/") {
      const idx = join(DIST, "index.html");
      if (!(await exists(idx))) broken.push({ from: relative(DIST, f), href });
      continue;
    }
    const path = join(DIST, base);
    if (!(await exists(path))) broken.push({ from: relative(DIST, f), href });
  }
}

console.log(`scanned ${total} hrefs across ${html.length} html files`);
console.log(`broken: ${broken.length}`);
for (const b of broken.slice(0, 50)) console.log(`  ${b.href}  (from ${b.from})`);
