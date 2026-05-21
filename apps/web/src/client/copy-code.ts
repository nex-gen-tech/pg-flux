/**
 * Copy-to-clipboard for every code block. The build pipeline wraps each
 * <pre> in a .code-block with a [data-copy-code] button carrying the plain
 * code text in an attribute, so the handler doesn't need to read the
 * highlighted DOM.
 */
function flashCopied(btn: HTMLButtonElement) {
  btn.classList.add("copied");
  const copyIcon = btn.querySelector<HTMLElement>(".code-block-copy-icon");
  const checkIcon = btn.querySelector<HTMLElement>(".code-block-check-icon");
  const label = btn.querySelector<HTMLElement>(".code-block-copy-label");
  copyIcon?.classList.add("hidden");
  checkIcon?.classList.remove("hidden");
  if (label) label.textContent = "Copied";
  window.setTimeout(() => {
    btn.classList.remove("copied");
    copyIcon?.classList.remove("hidden");
    checkIcon?.classList.add("hidden");
    if (label) label.textContent = "Copy";
  }, 1500);
}

async function copy(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    /* fall through to legacy path */
  }
  // Legacy execCommand fallback for older browsers / non-https.
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.style.position = "fixed";
  ta.style.left = "-9999px";
  document.body.appendChild(ta);
  ta.select();
  let ok = false;
  try {
    ok = document.execCommand("copy");
  } catch {
    ok = false;
  }
  document.body.removeChild(ta);
  return ok;
}

function init() {
  for (const btn of document.querySelectorAll<HTMLButtonElement>("[data-copy-code]")) {
    btn.addEventListener("click", async (e) => {
      e.preventDefault();
      const text = btn.getAttribute("data-copy-code") || "";
      if (await copy(text)) flashCopied(btn);
    });
  }
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
else init();
