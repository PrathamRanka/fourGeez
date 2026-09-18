import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

// cn combines conditional classes while resolving Tailwind conflicts.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
