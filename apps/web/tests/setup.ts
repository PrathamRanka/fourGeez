import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

if (typeof window.IntersectionObserver === "undefined") {
  window.IntersectionObserver = class IntersectionObserver {
    readonly root = null;
    readonly rootMargin = "0px";
    readonly thresholds = [0];

    disconnect() {}
    observe() {}
    takeRecords() {
      return [];
    }
    unobserve() {}
  };
}

afterEach(() => {
  cleanup();
});
