/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../test/setupDom";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, test, vi } from "vitest";
import { DashboardOverview } from "./DashboardOverview";
import { useDashboardStore, type DashboardViewState } from "../../stores/dashboardStore";

afterEach(() => {
  useDashboardStore.setState({ state: null, loading: false, error: null, lastLoadedAt: null });
  cleanup();
  vi.restoreAllMocks();
});

test("Dashboard 总览缺失涨跌分布字段时不渲染 NaN", () => {
  const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
  useDashboardStore.setState({
    state: {
      health: { status: "ok", version: "0.1.0" },
      summary: {
        watchlist: { up_count: 0, down_count: 0 },
        recent_reports: [],
        recent_tasks: [],
        market_news: [],
        risk_tips: [],
        provider_statuses: [],
      },
      indexQuotes: [],
      indexTrends: {},
      watchlistRows: [],
    } as unknown as DashboardViewState,
    loading: false,
    error: null,
    lastLoadedAt: "2026-06-23T00:00:00Z",
    load: vi.fn(),
  });

  render(
    <MemoryRouter>
      <DashboardOverview />
    </MemoryRouter>,
  );

  expect(screen.queryByText("NaN")).not.toBeInTheDocument();
  expect(consoleError).not.toHaveBeenCalledWith(expect.stringContaining("NaN"), expect.anything());
});
