/**
 * Dev server: rebuild on every request so local edits show up immediately.
 * Trades startup speed for development ergonomics.
 */
import { spawn } from "node:child_process";

async function rebuild() {
  return new Promise<void>((resolve, reject) => {
    const p = spawn("bun", ["run", "scripts/build.ts"], { stdio: "inherit" });
    p.on("exit", code => (code === 0 ? resolve() : reject(new Error("build failed"))));
  });
}

await rebuild();

const PORT = Number(process.env.PORT || 3000);

Bun.serve({
  port: PORT,
  async fetch(req) {
    // Rebuild every request — slow but always-fresh.
    await rebuild().catch(() => {});
    const url = new URL(req.url);
    let path = url.pathname;
    if (path === "/") path = "/index.html";
    if (!path.includes(".")) path = path.replace(/\/?$/, ".html");
    const file = Bun.file("./dist" + path);
    if (await file.exists()) return new Response(file);
    return new Response("Not Found", { status: 404 });
  },
});

console.log(`>> docs dev server on http://localhost:${PORT}`);
