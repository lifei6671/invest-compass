/* @vitest-environment jsdom */

import "@testing-library/jest-dom/vitest";
import "../../../test/setupDom";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test } from "vitest";
import { TechnicalIndicatorCard, buildIndicatorSummary } from "./TechnicalIndicatorCard";
import type { TechnicalIndicator } from "../types";

afterEach(() => {
  cleanup();
});

const indicators: TechnicalIndicator[] = [
  { name: "MA.MA5", value: "187.13", direction: "up", desc: "日K最新值" },
  { name: "MA.MA10", value: "181.20", direction: "up", desc: "日K最新值" },
  { name: "MACD.DIF", value: "26.18", direction: "up", desc: "日K最新值" },
  { name: "MACD.DEA", value: "23.22", direction: "up", desc: "日K最新值" },
  { name: "MACD.BAR", value: "5.90", direction: "up", desc: "日K最新值" },
  { name: "RSI.RSI6", value: "51.76", direction: "flat", desc: "日K最新值" },
  { name: "KDJ.K", value: "77.47", direction: "up", desc: "日K最新值" },
  { name: "KDJ.D", value: "81.49", direction: "up", desc: "日K最新值" },
  { name: "KDJ.J", value: "69.43", direction: "up", desc: "日K最新值" },
  { name: "BOLL.MID", value: "143.21", direction: "up", desc: "日K最新值" },
  { name: "BOLL.UPPER", value: "211.73", direction: "up", desc: "日K最新值" },
  { name: "BOLL.LOWER", value: "74.69", direction: "down", desc: "日K最新值" },
];

test("技术指标卡按当前分组聚合展示参数和值", () => {
  render(<TechnicalIndicatorCard items={indicators} />);

  expect(screen.getByText("MA(5,10,20,60)")).toBeInTheDocument();
  expect(screen.getByText("MA5")).toBeInTheDocument();
  expect(screen.queryByText("MACD.DIF")).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole("button", { name: "MACD" }));

  expect(screen.getByText("MACD(12,26,9)")).toBeInTheDocument();
  expect(screen.getByText("DIF")).toBeInTheDocument();
  expect(screen.getByText("DEA")).toBeInTheDocument();
  expect(screen.getByText("BAR")).toBeInTheDocument();
  expect(screen.getByText("26.18")).toBeInTheDocument();
  expect(screen.getByText("23.22")).toBeInTheDocument();
  expect(screen.getByText("5.90")).toBeInTheDocument();
  expect(screen.queryByText("MACD.DIF")).not.toBeInTheDocument();
});

test("指标摘要按 go-stock 风格保留指标参数", () => {
  const summary = buildIndicatorSummary("BOLL", indicators);

  expect(summary?.title).toBe("BOLL(20)");
  expect(summary?.values.map((item) => `${item.label}:${item.value}`)).toEqual([
    "MID:143.21",
    "UPPER:211.73",
    "LOWER:74.69",
  ]);
});
