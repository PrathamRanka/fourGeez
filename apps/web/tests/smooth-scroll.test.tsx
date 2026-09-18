import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SmoothScroll } from "@/components/site/smooth-scroll";

const { destroy, lenisConstructor } = vi.hoisted(() => ({
  destroy: vi.fn(),
  lenisConstructor: vi.fn(),
}));

vi.mock("lenis", () => ({
  default: class LenisMock {
    constructor(options: unknown) {
      lenisConstructor(options);
    }

    destroy() {
      destroy();
    }
  },
}));

describe("SmoothScroll", () => {
  it("adds accessible site-wide smoothing and cleans up on unmount", () => {
    const { unmount } = render(<SmoothScroll />);

    expect(lenisConstructor).toHaveBeenCalledWith(
      expect.objectContaining({
        anchors: expect.any(Object),
        autoRaf: true,
        respectReducedMotion: true,
        smoothWheel: true,
      }),
    );

    unmount();
    expect(destroy).toHaveBeenCalledOnce();
  });
});
