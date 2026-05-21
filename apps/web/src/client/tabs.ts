/**
 * Tabs — minimal, plain HTML, no Radix at runtime.
 * Markup contract:
 *   <div class="tabs" data-tabs>
 *     <div class="tab-list" role="tablist">
 *       <button role="tab" aria-selected="true"  data-tab="go">Go</button>
 *       <button role="tab" aria-selected="false" data-tab="ts">TypeScript</button>
 *     </div>
 *     <div class="tab-panel" role="tabpanel" data-tab-panel="go">...</div>
 *     <div class="tab-panel" role="tabpanel" data-tab-panel="ts" hidden>...</div>
 *   </div>
 */
function init() {
  for (const group of document.querySelectorAll<HTMLElement>("[data-tabs]")) {
    const triggers = group.querySelectorAll<HTMLButtonElement>("[data-tab]");
    const panels = group.querySelectorAll<HTMLElement>("[data-tab-panel]");
    triggers.forEach((trig) => {
      trig.addEventListener("click", () => {
        const key = trig.dataset.tab;
        triggers.forEach((t) => t.setAttribute("aria-selected", String(t.dataset.tab === key)));
        panels.forEach((p) => {
          if (p.dataset.tabPanel === key) p.removeAttribute("hidden");
          else p.setAttribute("hidden", "");
        });
      });
    });
  }
}
if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
else init();
