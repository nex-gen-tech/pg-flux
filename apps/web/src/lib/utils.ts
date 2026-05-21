import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/** cn — combine classnames with tailwind-merge to handle conflicting utilities. */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
