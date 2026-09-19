"use client";

import { useEffect, useState } from "react";
import { ChevronDown, Menu, X } from "lucide-react";
import styles from "./docs.module.css";

export type DocsNavGroup = {
  label: string;
  items: Array<{ id: string; label: string; nested?: boolean }>;
};

function NavigationLinks({
  groups,
  activeId,
  onNavigate,
}: {
  groups: DocsNavGroup[];
  activeId: string;
  onNavigate?: (id: string) => void;
}) {
  return (
    <nav aria-label="Documentation navigation">
      {groups.map((group) => (
        <div className={styles.navGroup} key={group.label}>
          <p>{group.label}</p>
          {group.items.map((item) => (
            <a
              className={item.nested ? styles.nestedLink : undefined}
              href={`#${item.id}`}
              aria-current={activeId === item.id ? "location" : undefined}
              onClick={() => onNavigate?.(item.id)}
              key={item.id}
            >
              {item.label}
            </a>
          ))}
        </div>
      ))}
    </nav>
  );
}

export function DocsNavigation({
  groups,
  mobile = false,
}: {
  groups: DocsNavGroup[];
  mobile?: boolean;
}) {
  const [activeId, setActiveId] = useState("quickstart");
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!("IntersectionObserver" in window)) return;
    const ids = groups.flatMap((group) => group.items.map((item) => item.id));
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries.find((entry) => entry.isIntersecting);
        if (visible?.target.id) setActiveId(visible.target.id);
      },
      { rootMargin: "-18% 0px -68% 0px" },
    );
    ids.forEach((id) => {
      const section = document.getElementById(id);
      if (section) observer.observe(section);
    });
    return () => observer.disconnect();
  }, [groups]);

  if (!mobile) {
    return (
      <NavigationLinks
        groups={groups}
        activeId={activeId}
        onNavigate={setActiveId}
      />
    );
  }

  return (
    <div className={styles.mobileNavigation}>
      <button
        type="button"
        aria-expanded={open}
        aria-controls="mobile-docs-navigation"
        aria-label={
          open
            ? "Close documentation navigation"
            : "Open documentation navigation"
        }
        onClick={() => setOpen((current) => !current)}
      >
        {open ? <X aria-hidden="true" /> : <Menu aria-hidden="true" />}
        <span>Documentation</span>
        <ChevronDown aria-hidden="true" />
      </button>
      {open ? (
        <div id="mobile-docs-navigation">
          <NavigationLinks
            groups={groups}
            activeId={activeId}
            onNavigate={(id) => {
              setActiveId(id);
              setOpen(false);
            }}
          />
        </div>
      ) : null}
    </div>
  );
}
