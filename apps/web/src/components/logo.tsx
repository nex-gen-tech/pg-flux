import * as React from "react";

/**
 * pg-flux mark — abstract glyph nodding to the Postgres elephant silhouette
 * without infringing the trademark. Two interlocking shapes: a database
 * cylinder (DDL) and a flowing curve (migration). Solid, no gradient.
 */
export function Logo({ className, size = 22 }: { className?: string; size?: number }) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <rect
        x="3"
        y="5"
        width="13"
        height="14"
        rx="2.5"
        stroke="currentColor"
        strokeWidth="1.75"
      />
      <path
        d="M3 9.5 H 16 M 3 14.5 H 16"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
      <path
        d="M18 8 C 21 9.5, 21 14.5, 18 16"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
        fill="none"
      />
      <circle cx="20.25" cy="12" r="1.4" fill="currentColor" />
    </svg>
  );
}
