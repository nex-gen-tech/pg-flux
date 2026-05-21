import * as React from "react";

/**
 * Animated macOS terminal — pure CSS keyframes, no client JS.
 * Each line is its own .term-line with a stagger delay class. The blinking
 * cursor lives on the final line so it reads as "session in progress".
 */
export function TerminalHero() {
  return (
    <div className="term w-full">
      <div className="term-bar">
        <span className="term-dot term-dot--red" />
        <span className="term-dot term-dot--yellow" />
        <span className="term-dot term-dot--green" />
        <span className="term-title">~/app · pg-flux · zsh</span>
      </div>
      <div className="term-body">
        <span className="term-line delay-1">
          <span className="term-prompt">$ </span>pg-flux init
        </span>
        <span className="term-line delay-2 term-dim">
          ✓ scaffold .pg-flux.yml, schema/, migrations/
        </span>

        <span className="term-line delay-3">
          <span className="term-prompt">$ </span>pg-flux migrate generate --label add_users
        </span>
        <span className="term-line delay-4 term-dim">
          Generated: migrations/20260520_add_users.sql (4 statements)
        </span>

        <span className="term-line delay-5">
          <span className="term-prompt">$ </span>pg-flux migrate apply
        </span>
        <span className="term-line delay-6 term-dim">
          apply 20260520_add_users.sql … ok
        </span>

        <span className="term-line delay-7">
          <span className="term-prompt">$ </span>pg-flux gen --lang go,ts --validators=zod
        </span>
        <span className="term-line delay-8 term-dim">
          [go] wrote 5 files · [ts] wrote 8 files
          <span className="term-cursor" aria-hidden />
        </span>
      </div>
    </div>
  );
}
