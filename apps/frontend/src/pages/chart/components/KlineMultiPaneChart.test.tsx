/* @vitest-environment jsdom */

import { expect, test } from "vitest";
import {
  buildIndicatorHoverTooltipData,
  buildCandleTooltipLegendTemplate,
  buildKLineChartStyles,
  buildKLineChartLayout,
  klineChartLocale,
  toKLineChartData,
  toKLineChartPeriod,
} from "./KlineMultiPaneChart";
import type { IndicatorSeries, KlineBar } from "../types";

const bars: KlineBar[] = [
  { date: "2026-06-25 09:30", open: 96.3, high: 96.3, low: 96.3, close: 96.3, volume: 120000 },
  { date: "2026-06-25 09:35", open: 95.8, high: 95.8, low: 95.8, close: 95.8, volume: 140000 },
  { date: "2026-06-25 09:40", open: 94.2, high: 94.2, low: 94.2, close: 94.2, volume: 180000 },
  { date: "2026-06-25 09:45", open: 93.78, high: 93.78, low: 93.78, close: 93.78, volume: 200000 },
];

const indicators: IndicatorSeries = {
  ma5: [null, 96.05, 95.43, 95.02],
  ma10: [null, 96.1, 95.6, 95.4],
  ma20: [null, 96.2, 95.9, 95.8],
  ma60: [null, 96.3, 96.2, 96.1],
  bollUp: [null, 97.2, 96.7, 96.1],
  bollMid: [null, 96.2, 95.7, 95.1],
  bollDn: [null, 95.2, 94.7, 94.1],
  volMa5: [null, 130000, 146666.67, 160000],
  volMa10: [null, 128000, 142000, 150000],
  macdDif: [0.2, -0.1, -0.3, -0.2],
  macdDea: [0.1, 0, -0.2, -0.1],
  macdHist: [0.2, -0.2, -0.2, -0.1],
  kdjK: [60, 42, 28, 78],
  kdjD: [50, 40, 34, 60],
  kdjJ: [120, -18, 16, 114],
  rsi6: [55, 48, 40, 36],
};

test("转换为 klinecharts 所需的 OHLC 数据", () => {
  const data = toKLineChartData(bars, indicators);

  expect(data[0]).toMatchObject({
    open: 96.3,
    high: 96.3,
    low: 96.3,
    close: 96.3,
    volume: 120000,
  });
  expect(data[0].timestamp).toBe(new Date("2026-06-25T09:30:00+08:00").getTime());
});

test("K 线数据携带浮层展示所需的指标值", () => {
  const data = toKLineChartData(bars, indicators);

  expect(data[3]).toMatchObject({
    ma5Text: "95.02",
    ma60Text: "96.10",
    bollUpText: "96.10",
    volMa5Text: "16.00万",
    macdDifText: "-0.2000",
    macdHistText: "-0.1000",
    kdjJText: "114.0000",
    rsi6Text: "36.0000",
  });
});

test("分时周期映射为 klinecharts 的 1 分钟周期", () => {
  expect(toKLineChartPeriod("minute")).toEqual({ type: "minute", span: 1 });
  expect(toKLineChartPeriod("5m")).toEqual({ type: "minute", span: 5 });
  expect(toKLineChartPeriod("day")).toEqual({ type: "day", span: 1 });
  expect(toKLineChartPeriod("quarter")).toEqual({ type: "month", span: 3 });
  expect(toKLineChartPeriod("year")).toEqual({ type: "year", span: 1 });
});

test("根据激活指标生成 klinecharts 多窗格布局", () => {
  const layout = buildKLineChartLayout(["MA", "VOL", "MACD", "KDJ"], "day");
  const panes = layout.panes ?? [];

  expect(panes[0]).toEqual(
    expect.objectContaining({ type: "candle", content: [expect.objectContaining({ name: "MA", calcParams: [5, 10, 20, 60] })] }),
  );
  expect(panes).toEqual(expect.arrayContaining([
    expect.objectContaining({ type: "indicator", content: [expect.objectContaining({ name: "VOL", calcParams: [5, 10] })] }),
    expect.objectContaining({ type: "indicator", content: ["MACD"] }),
    expect.objectContaining({ type: "indicator", content: ["KDJ"] }),
    expect.objectContaining({ type: "xAxis" }),
  ]));
});

test("go-stock 指标映射到主图和副图窗格", () => {
  const layout = buildKLineChartLayout([
    "MA",
    "EMA",
    "KAMA",
    "STREND",
    "SAR",
    "ICHI",
    "DEMA",
    "SATS",
    "GATOR",
    "HULL",
    "TEMA",
    "BOLL",
    "KELT",
    "DONCH",
    "VWAP",
    "VWBAND",
    "PIVOT",
    "ZIGZAG",
    "FRACTAL",
    "SMC",
    "VOL",
    "CCI",
    "AO",
    "TRIX",
    "ROC",
    "PVT",
    "ADX",
  ], "day");
  const panes = layout.panes ?? [];

  expect(panes[0]).toEqual(
    expect.objectContaining({
      type: "candle",
      content: expect.arrayContaining([
        expect.objectContaining({ name: "MA" }),
        expect.objectContaining({ name: "EMA" }),
        "KAMA",
        "STREND",
        "SAR",
        "ICHI",
        "DEMA",
        "SATS",
        "GATOR",
        "HULL",
        "TEMA",
        "BOLL",
        "KELT",
        "DONCH",
        "VWAP",
        "VWBAND",
        "PIVOT",
        "ZIGZAG",
        "FRACTAL",
        "SMC",
      ]),
    }),
  );
  expect(panes).toEqual(expect.arrayContaining([
    expect.objectContaining({ type: "indicator", content: [expect.objectContaining({ name: "VOL", calcParams: [5, 10] })] }),
    expect.objectContaining({ type: "indicator", content: ["CCI"] }),
    expect.objectContaining({ type: "indicator", content: ["AO"] }),
    expect.objectContaining({ type: "indicator", content: ["TRIX"] }),
    expect.objectContaining({ type: "indicator", content: ["ROC"] }),
    expect.objectContaining({ type: "indicator", content: ["PVT"] }),
    expect.objectContaining({ type: "indicator", content: ["ADX"] }),
    expect.objectContaining({ type: "xAxis" }),
  ]));
});

test("klinecharts 使用中文行情术语", () => {
  expect(klineChartLocale).toBe("zh-CN");
});

test("浮层模板只展示当前蜡烛基础行情", () => {
  const template = buildCandleTooltipLegendTemplate();
  const titles = template.map((item) => (typeof item.title === "string" ? item.title : item.title.text));

  expect(titles).toEqual(["时间：", "开：", "高：", "低：", "收：", "成交量："]);
  expect(titles).not.toContain("DIF：");
  expect(titles).not.toContain("K：");
});

test("主图启用 klinecharts 原生中文 tooltip", () => {
  const styles = buildKLineChartStyles();

  expect(styles.candle?.tooltip?.showRule).toBe("follow_cross");
  expect(styles.candle?.tooltip?.legend?.template).toEqual(buildCandleTooltipLegendTemplate());
});

test("分时主图使用分时线并只挂均线", () => {
  const styles = buildKLineChartStyles("minute");
  const layout = buildKLineChartLayout(["BOLL", "SAR", "MACD"], "minute");
  const panes = layout.panes ?? [];

  expect(styles.candle?.type).toBe("area");
  expect(panes[0]).toEqual(
    expect.objectContaining({ type: "candle", content: [expect.objectContaining({ name: "MA", calcParams: [5, 10, 20] })] }),
  );
  expect(panes[0]).not.toEqual(expect.objectContaining({ content: expect.arrayContaining(["BOLL", "SAR"]) }));
});

test("非分时主图仍使用蜡烛图样式", () => {
  expect(buildKLineChartStyles("5m").candle?.type).toBe("candle_solid");
  expect(buildKLineChartStyles("day").candle?.type).toBe("candle_solid");
});

test("副图浮层按指标窗格展示当前值", () => {
  const data = buildIndicatorHoverTooltipData(
    [
      {
        name: "VOL",
        precision: 0,
        shouldFormatBigNumber: true,
        figures: [
          { key: "ma1", title: "MA5: " },
          { key: "ma2", title: "MA10: " },
          { key: "volume", title: "VOLUME: " },
        ],
        result: [{ ma1: 106700, ma2: 101800, volume: 94100 }],
      },
      {
        name: "MACD",
        precision: 4,
        shouldFormatBigNumber: false,
        figures: [
          { key: "dif", title: "DIF: " },
          { key: "dea", title: "DEA: " },
          { key: "macd", title: "MACD: " },
        ],
        result: [{ dif: -0.091, dea: 0.0468, macd: -0.2756 }],
      },
      {
        name: "KDJ",
        precision: 4,
        shouldFormatBigNumber: false,
        figures: [
          { key: "k", title: "K: " },
          { key: "d", title: "D: " },
          { key: "j", title: "J: " },
        ],
        result: [{ k: 22.6711, d: 29.6991, j: 8.6151 }],
      },
    ],
    0,
  );

  expect(data.VOL).toEqual([
    expect.objectContaining({ title: "MA5：", value: "10.67万" }),
    expect.objectContaining({ title: "MA10：", value: "10.18万" }),
    expect.objectContaining({ title: "成交量：", value: "9.41万" }),
  ]);
  expect(data.MACD).toEqual([
    expect.objectContaining({ title: "DIF：", value: "-0.0910" }),
    expect.objectContaining({ title: "DEA：", value: "0.0468" }),
    expect.objectContaining({ title: "MACD：", value: "-0.2756" }),
  ]);
  expect(data.KDJ).toEqual([
    expect.objectContaining({ title: "K：", value: "22.6711" }),
    expect.objectContaining({ title: "D：", value: "29.6991" }),
    expect.objectContaining({ title: "J：", value: "8.6151" }),
  ]);
});
