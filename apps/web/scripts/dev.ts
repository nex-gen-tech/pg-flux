/**
 * Dev server — rebuild on every request. Slow but always fresh; ideal for
 * editing markdown content + iterating on components.
 */
import { spawnSync } from "node:child_process";

function rebuild(): boolean {
  const r = spawnSync("bun", ["run", "scripts/build.tsx"], { stdio: "inherit" });
  return r.status === 0;
}

if (!rebuild()) {
  process.exit(1);
}

const PORT = Number(process.env.PORT || 3000);

Bun.serve({
  port: PORT,
  async fetch(req) {
    const url = new URL(req.url);
    let path = url.pathname;
    // Only rebuild on document requests, not asset hits.
    if (path === "/" || path.endsWith(".html") || !path.includes(".")) {
      rebuild();
    }
    if (path === "/") path = "/index.html";
    if (!path.includes(".")) path = path.replace(/\/?$/, ".html");
    const file = Bun.file("./dist" + path);
    if (await file.exists()) return new Response(file);
    return new Response("Not Found", { status: 404 });
  },
});

console.log(`>> docs dev server on http://localhost:${PORT}`);
