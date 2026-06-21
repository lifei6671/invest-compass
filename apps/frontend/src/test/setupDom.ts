class MockResizeObserver implements ResizeObserver {
  observe() {}

  unobserve() {}

  disconnect() {}
}

if (typeof window !== "undefined" && typeof window.ResizeObserver === "undefined") {
  window.ResizeObserver = MockResizeObserver;
}

if (typeof globalThis.ResizeObserver === "undefined") {
  globalThis.ResizeObserver = MockResizeObserver;
}
