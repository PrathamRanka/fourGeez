"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

type ApplicationShellProps = {
  children: ReactNode;
};

// ApplicationShell keeps public navigation out of the authenticated dashboard workspace.
export function ApplicationShell({ children }: ApplicationShellProps) {
  const pathname = usePathname();

  if (pathname.startsWith("/dashboard")) {
    return children;
  }

  return (
    <div className="public-shell">
      {children}
    </div>
  );
}
