/* @vitest-environment jsdom */

import { getSupportedIndicators } from "klinecharts";
import { expect, test } from "vitest";
import {
  goStockCandleIndicators,
  goStockIndicatorGroups,
  goStockIndicatorKeys,
  goStockPaneIndicators,
  registerGoStockIndicators,
  resolveIndicatorBarStyle,
} from "./goStockIndicators";

test("go-stock 指标分组全部接入可注册指标", () => {
  registerGoStockIndicators();

  const supported = getSupportedIndicators();
  for (const key of goStockIndicatorKeys) {
    expect(supported).toContain(key);
  }
  expect(goStockIndicatorGroups.flatMap((group) => group.items)).toHaveLength(50);
});

test("go-stock 指标按主图和副图分流", () => {
  expect(goStockCandleIndicators).toEqual(
    expect.arrayContaining(["MA", "EMA", "KAMA", "BOLL", "KELT", "DONCH", "VWAP", "VWBAND", "PIVOT", "SMC"]),
  );
  expect(goStockPaneIndicators).toEqual(
    expect.arrayContaining(["MACD", "KDJ", "RSI", "CCI", "WR", "SRSI", "CMO", "MFI", "CMF", "ADX", "SIGNALRATIO"]),
  );
});

test("柱形指标按信号方向着色", () => {
  expect(resolveIndicatorBarStyle(1)).toMatchObject({ color: "#ff4d4f", borderColor: "#ff4d4f" });
  expect(resolveIndicatorBarStyle(-1)).toMatchObject({ color: "#16a34a", borderColor: "#16a34a" });
  expect(resolveIndicatorBarStyle(0)).toMatchObject({ color: "#94a3b8", borderColor: "#94a3b8" });
});
