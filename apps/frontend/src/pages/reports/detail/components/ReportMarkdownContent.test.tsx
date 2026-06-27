/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import { ReportMarkdownContent } from "./ReportMarkdownContent";
import { openExternalURL } from "../../../../services/coreClient";

vi.mock("../../../../services/coreClient", () => ({
  openExternalURL: vi.fn().mockResolvedValue({ opened: true }),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

test("报告 Markdown 外链通过固定桌面 command 打开", async () => {
  render(<ReportMarkdownContent markdown="参考 [公告](https://example.com/report)" />);

  fireEvent.click(screen.getByRole("link", { name: "公告" }));

  await waitFor(() => {
    expect(openExternalURL).toHaveBeenCalledWith("https://example.com/report");
  });
});

test("报告 Markdown 不渲染非 HTTPS 外链", () => {
  render(<ReportMarkdownContent markdown="联系 [作者](mailto:test@example.com)，或访问 [站点](http://example.com)" />);

  expect(screen.queryByRole("link", { name: "作者" })).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "站点" })).not.toBeInTheDocument();
});
