/**
 * Preview the static build. Reads from dist/ without rebuilding.
 */
const PORT = Number(process.env.PORT || 4000);

Bun.serve({
  port: PORT,
  async fetch(req) {
    const url = new URL(req.url);
    let path = url.pathname;
    if (path === "/") path = "/index.html";
    if (!path.includes(".")) path = path.replace(/\/?$/, ".html");
    const file = Bun.file("./dist" + path);
    if (await file.exists()) return new Response(file);
    return new Response("Not Found", { status: 404 });
  },
});

console.log(`>> static preview on http://localhost:${PORT}`);
