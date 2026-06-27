/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { expect, test } from "vitest";
import { buildKlineOption } from "./KlineChartCard";
import type { KlineItem } from "../types";

const klineItems: KlineItem[] = [
  { date: "2026-03-01", open: 37.89, close: 36.99, low: 36.75, high: 38.8, volume: 123456789 },
  { date: "2026-03-02", open: 36.99, close: 37.21, low: 36.5, high: 37.5, volume: 98765432 },
];

test("股票详情K线图使用中文 tooltip 并为右侧坐标轴预留空间", () => {
  const option = buildKlineOption(klineItems) as {
    grid: Array<{ right: number; containLabel: boolean }>;
    tooltip: { formatter: (params: unknown) => string };
    yAxis: Array<{ axisLabel: { formatter: (value: number) => string } }>;
  };

  const tooltip = option.tooltip.formatter([{ dataIndex: 0 }]);

  expect(tooltip).toContain("开盘");
  expect(tooltip).toContain("收盘");
  expect(tooltip).toContain("最低");
  expect(tooltip).toContain("最高");
  expect(tooltip).toContain("成交量");
  expect(tooltip).not.toContain("open");
  expect(tooltip).not.toContain("close");
  expect(option.grid[0].right).toBeGreaterThanOrEqual(60);
  expect(option.grid[1].containLabel).toBe(true);
  expect(option.yAxis[1].axisLabel.formatter(123456789)).toBe("1.23亿");
});
