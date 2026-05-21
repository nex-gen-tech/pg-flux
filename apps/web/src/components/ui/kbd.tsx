import * as React from "react";
import { cn } from "@/lib/utils";

export function Kbd({ className, ...props }: React.HTMLAttributes<HTMLSpanElement>) {
  return (
    <kbd
      className={cn(
        "inline-flex h-5 select-none items-center gap-1 rounded border bg-[--color-muted] px-1.5 font-mono text-[10px] font-medium text-[--color-muted-foreground]",
        className,
      )}
      {...props}
    />
  );
}
