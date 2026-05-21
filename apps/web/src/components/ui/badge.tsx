import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium transition-colors",
  {
    variants: {
      variant: {
        default:
          "border-transparent bg-[--color-primary] text-[--color-primary-foreground]",
        secondary:
          "border-transparent bg-[--color-secondary] text-[--color-secondary-foreground]",
        outline:
          "border-[--color-border] text-[--color-foreground]",
        accent:
          "border-transparent bg-[--color-accent]/15 text-[--color-accent]",
      },
    },
    defaultVariants: { variant: "default" },
  },
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />;
}
