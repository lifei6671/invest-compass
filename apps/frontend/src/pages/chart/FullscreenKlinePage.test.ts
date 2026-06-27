/* @vitest-environment jsdom */

import { expect, test } from "vitest";
import { filterLatestTradingDayBars, resolveIndexDisplayName } from "./FullscreenKlinePage";
import type { KlineBar } from "./types";

test("分时数据只保留最新一个交易日", () => {
  const bars: KlineBar[] = [
    { date: "2026-06-24 14:58", open: 7.6, high: 7.62, low: 7.58, close: 7.61, volume: 1000 },
    { date: "2026-06-24 14:59", open: 7.61, high: 7.63, low: 7.6, close: 7.62, volume: 1100 },
    { date: "2026-06-25 09:30", open: 7.7, high: 7.76, low: 7.68, close: 7.75, volume: 1200 },
    { date: "2026-06-25 09:31", open: 7.75, high: 7.82, low: 7.74, close: 7.8, volume: 1300 },
  ];

  expect(filterLatestTradingDayBars(bars).map((bar) => bar.date)).toEqual(["2026-06-25 09:30", "2026-06-25 09:31"]);
});

test("分时数据支持紧凑日期格式识别最新交易日", () => {
  const bars: KlineBar[] = [
    { date: "20260624 1459", open: 7.6, high: 7.62, low: 7.58, close: 7.61, volume: 1000 },
    { date: "20260625 0930", open: 7.7, high: 7.76, low: 7.68, close: 7.75, volume: 1200 },
  ];

  expect(filterLatestTradingDayBars(bars).map((bar) => bar.date)).toEqual(["20260625 0930"]);
});

test("指数标题在缺少 profile 名称时按交易所代码兜底显示", () => {
  expect(resolveIndexDisplayName("CN:SH:000001")).toBe("上证指数");
  expect(resolveIndexDisplayName("000001.SH")).toBe("上证指数");
  expect(resolveIndexDisplayName("CN:SZ:000001")).toBe("");
});
