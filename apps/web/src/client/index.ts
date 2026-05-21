/**
 * Client-side interactivity for the static docs site.
 * No framework runtime — vanilla TS targeting modern browsers.
 *
 *  - Theme toggle (light/dark/system) with localStorage persistence
 *  - ⌘K / Ctrl+K search modal with fuzzy match
 *  - Mobile sidebar (slide-in drawer + backdrop)
 *  - Active-section scrollspy for the right-rail TOC
 */
import "./theme";
import "./search";
import "./mobile-nav";
import "./toc-scrollspy";
import "./copy-code";
import "./tabs";
