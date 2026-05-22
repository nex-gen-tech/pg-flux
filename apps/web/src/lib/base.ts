// BASE_PATH is set at build time (e.g. "/pg-flux" for GitHub Pages).
// Leave empty for local dev / custom domain serving from root.
export const BASE: string = (typeof process !== "undefined" && process.env?.BASE_PATH) ?? "";
