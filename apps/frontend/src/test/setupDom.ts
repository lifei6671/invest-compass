class TestResizeObserver implements ResizeObserver {
  observe() {}

  unobserve() {}

  disconnect() {}
}

if (typeof window !== "undefined" && typeof window.ResizeObserver === "undefined") {
  window.ResizeObserver = TestResizeObserver;
}

if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = TestResizeObserver;
}
