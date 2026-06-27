import {
  registerIndicator,
  type IndicatorFigure,
  type IndicatorFigureStyle,
  type IndicatorTemplate,
  type KLineData,
} from "klinecharts";
import type { IndicatorKey } from "./types";

type NullableNumber = number | null;
type IndicatorRow = Record<string, number | boolean | null | undefined>;
const indicatorUpColor = "#ff4d4f";
const indicatorDownColor = "#16a34a";
const indicatorNeutralColor = "#94a3b8";

export type IndicatorCatalogItem = {
  label: string;
  key: IndicatorKey;
};

export type IndicatorCatalogGroup = {
  title: string;
  tone: string;
  items: IndicatorCatalogItem[];
};

export const quickIndicators: IndicatorKey[] = ["MA", "EMA", "BOLL", "MACD", "KDJ", "RSI", "VOL", "WR", "OBV"];

export const goStockIndicatorTips: Record<IndicatorKey, { effect: string; formula: string }> = {
  MA: { effect: "识别价格均线趋势和支撑压力。", formula: "N 日收盘价简单平均，默认 MA5/10/20/60。" },
  EMA: { effect: "更快跟随价格变化的趋势线。", formula: "指数移动平均，最新价格权重更高，默认 EMA5/10/20/60。" },
  KAMA: { effect: "根据行情噪声自适应快慢的均线。", formula: "效率比 ER 调整平滑系数，KAMA=前值+SC*(收盘价-前值)。" },
  STREND: { effect: "判断趋势方向和趋势止损位置。", formula: "HL2 加减 ATR 倍数形成上下轨，价格突破后切换方向。" },
  SAR: { effect: "跟踪趋势反转点。", formula: "抛物线 SAR=前 SAR+AF*(EP-前 SAR)，AF 随极值推进递增。" },
  ICHI: { effect: "综合趋势、支撑压力和滞后确认。", formula: "转换线/基准线/云带由不同周期最高价和最低价均值计算。" },
  AROON: { effect: "衡量新高和新低出现的远近。", formula: "AroonUp/Down=(周期-距最高/最低点天数)/周期*100。" },
  DEMA: { effect: "降低 EMA 滞后的双指数均线。", formula: "DEMA=2*EMA(close)-EMA(EMA(close))。" },
  SATS: { effect: "自适应趋势跟踪线。", formula: "以 Supertrend/ATR 为基础，结合波动与量能做自适应带宽。" },
  GATOR: { effect: "显示多周期平滑线的趋势张合。", formula: "Alligator Jaw/Teeth/Lips 使用不同周期均线并向后偏移。" },
  HULL: { effect: "更平滑且滞后较低的均线。", formula: "HMA=WMA(2*WMA(n/2)-WMA(n), sqrt(n))。" },
  TEMA: { effect: "三重指数均线，进一步降低滞后。", formula: "TEMA=EMA1+(EMA1-EMA2)+(EMA1-EMA2-(EMA2-EMA3))。" },
  BOLL: { effect: "观察价格波动区间和突破。", formula: "中轨=SMA， 上/下轨=中轨 ± 标准差倍数。" },
  KELT: { effect: "用 ATR 描述趋势通道。", formula: "中轨=EMA，上/下轨=EMA ± ATR*倍数。" },
  DONCH: { effect: "观察阶段最高价和最低价通道。", formula: "上轨=N 周期最高价，下轨=N 周期最低价，中轨=(上轨+下轨)/2。" },
  ATR: { effect: "衡量真实波动幅度。", formula: "TR=max(高低差, |高-昨收|, |低-昨收|)，ATR 为 TR 平滑均值。" },
  AVGAMP: { effect: "衡量近期平均振幅。", formula: "振幅=(高-低)/收盘价*100，再取 5/10/20 周期均值。" },
  TTM: { effect: "观察波动压缩后的动量释放。", formula: "比较 BOLL 与 Keltner 通道，动量用典型价偏离 EMA 表示。" },
  ZIGZAG: { effect: "过滤小波动，突出主要拐点。", formula: "价格反向变动超过阈值后记录高低点。" },
  FRACTAL: { effect: "识别局部高点和低点。", formula: "当前高/低点高于或低于左右 N 根 K 线时成立。" },
  MASS: { effect: "用高低价波幅判断潜在反转。", formula: "Mass=高低价差 EMA 与二次 EMA 比值的周期和。" },
  SMC: { effect: "标记结构高低点，辅助观察 BOS/CHOCH。", formula: "按 internal/swing 周期寻找局部高低点。" },
  MACD: { effect: "识别趋势动能和背离。", formula: "DIF=EMA12-EMA26，DEA=DIF 的 EMA9，柱=DIF-DEA 的 2 倍。" },
  KDJ: { effect: "判断短线超买超卖和拐点。", formula: "RSV=(收-周期低)/(周期高-周期低)*100，K/D 平滑，J=3K-2D。" },
  RSI: { effect: "衡量上涨和下跌力量对比。", formula: "RSI=100-100/(1+平均涨幅/平均跌幅)。" },
  CCI: { effect: "衡量价格偏离典型价均值程度。", formula: "CCI=(TP-MA(TP))/(0.015*平均绝对偏差)。" },
  WR: { effect: "判断价格在近期高低区间的位置。", formula: "WR=(周期最高价-收盘价)/(最高价-最低价)*-100。" },
  SRSI: { effect: "提高 RSI 对短期变化的敏感度。", formula: "对 RSI 再做随机指标计算，并平滑得到 K/D。" },
  CMO: { effect: "衡量上涨动量和下跌动量差值。", formula: "CMO=(上涨和-下跌和)/(上涨和+下跌和)*100。" },
  AO: { effect: "观察短中期动量差。", formula: "AO=SMA(中价,5)-SMA(中价,34)。" },
  TRIX: { effect: "观察三重平滑后的趋势动量。", formula: "TRIX=EMA3 的变化率，Slope 为一阶差分。" },
  ROC: { effect: "衡量价格相对 N 周期前的变化率。", formula: "ROC=(当前收盘-N 周期前收盘)/N 周期前收盘*100。" },
  SMI: { effect: "随机动量指标，噪声低于传统随机指标。", formula: "收盘价相对周期高低中点的位置，经 EMA 平滑得到 SMI/Signal。" },
  COPPCK: { effect: "观察中长期动量拐点。", formula: "Coppock=WMA(ROC14+ROC11,10)。" },
  VOL: { effect: "观察成交量和量能均线。", formula: "柱为成交量，MA5/MA10 为成交量均线。" },
  OBV: { effect: "用成交量累积判断资金方向。", formula: "上涨日加成交量，下跌日减成交量，平盘不变。" },
  PVT: { effect: "结合涨跌幅和成交量观察量价趋势。", formula: "PVT 累加 ((收-昨收)/昨收)*成交量。" },
  VWAP: { effect: "观察成交量加权平均价格。", formula: "VWAP=sum(典型价*成交量)/sum(成交量)。" },
  MFI: { effect: "用价格和成交量判断资金流强弱。", formula: "按典型价变化区分正负资金流，MFI=100-100/(1+正/负资金流)。" },
  CMF: { effect: "衡量资金流入流出强度。", formula: "CMF=sum(资金流量乘数*成交量)/sum(成交量)。" },
  FI: { effect: "衡量价格变化背后的成交量推动力。", formula: "Force=(收-昨收)*成交量，再做 EMA 平滑。" },
  AD: { effect: "累积资金流向。", formula: "A/D 累加资金流量乘数*成交量。" },
  CHKOSC: { effect: "观察 A/D 线的快慢均线差。", formula: "Chaikin Osc=EMA(A/D,3)-EMA(A/D,10)。" },
  VWBAND: { effect: "观察 VWAP 附近的波动带。", formula: "VWAP 上下叠加成交量加权标准差倍数。" },
  ADX: { effect: "衡量趋势强度，不直接判断方向。", formula: "由 +DI/-DI 和 DX 平滑得到 ADX。" },
  PIVOT: { effect: "用上一周期价格估算支撑压力。", formula: "PP=(高+低+收)/3，R/S 位由 PP 推导。" },
  CHOP: { effect: "判断行情趋势性或震荡性。", formula: "CHOP=100*log(sum(TR)/(周期高-周期低))/log(N)。" },
  ELDER: { effect: "观察多空力量相对均线的强弱。", formula: "Bull=最高价-EMA，Bear=最低价-EMA。" },
  ULCER: { effect: "衡量回撤深度和持续压力。", formula: "周期内相对最高收盘价的回撤平方均值开方。" },
  SIGNALRATIO: { effect: "汇总多指标的多空倾向。", formula: "统计 MACD/KDJ/RSI/CCI/OBV/ST 等信号，输出看多/看空/净信号比例。" },
  DMI: { effect: "兼容旧 ADX/DMI 指标入口。", formula: "同 ADX：由 +DI/-DI 和 DX 平滑得到趋势强度。" },
};

export const goStockIndicatorGroups: IndicatorCatalogGroup[] = [
  {
    title: "趋势",
    tone: "#ef4444",
    items: [
      { label: "MA", key: "MA" },
      { label: "EMA", key: "EMA" },
      { label: "KAMA", key: "KAMA" },
      { label: "STrend", key: "STREND" },
      { label: "SAR", key: "SAR" },
      { label: "Ichi", key: "ICHI" },
      { label: "Aroon", key: "AROON" },
      { label: "DEMA", key: "DEMA" },
      { label: "SATS", key: "SATS" },
      { label: "Gator", key: "GATOR" },
      { label: "Hull", key: "HULL" },
      { label: "TEMA", key: "TEMA" },
    ],
  },
  {
    title: "波动",
    tone: "#f97316",
    items: [
      { label: "BOLL", key: "BOLL" },
      { label: "Kelt", key: "KELT" },
      { label: "Donch", key: "DONCH" },
      { label: "ATR", key: "ATR" },
      { label: "均幅", key: "AVGAMP" },
      { label: "TTM", key: "TTM" },
      { label: "ZigZag", key: "ZIGZAG" },
      { label: "Fractal", key: "FRACTAL" },
      { label: "Mass", key: "MASS" },
      { label: "SMC", key: "SMC" },
    ],
  },
  {
    title: "动量",
    tone: "#3b82f6",
    items: [
      { label: "MACD", key: "MACD" },
      { label: "KDJ", key: "KDJ" },
      { label: "RSI", key: "RSI" },
      { label: "CCI", key: "CCI" },
      { label: "W%R", key: "WR" },
      { label: "SRSI", key: "SRSI" },
      { label: "CMO", key: "CMO" },
      { label: "AO", key: "AO" },
      { label: "TRIX", key: "TRIX" },
      { label: "ROC", key: "ROC" },
      { label: "SMI", key: "SMI" },
      { label: "Coppck", key: "COPPCK" },
    ],
  },
  {
    title: "量价",
    tone: "#22c55e",
    items: [
      { label: "VOL", key: "VOL" },
      { label: "OBV", key: "OBV" },
      { label: "PVT", key: "PVT" },
      { label: "VWAP", key: "VWAP" },
      { label: "MFI", key: "MFI" },
      { label: "CMF", key: "CMF" },
      { label: "FI", key: "FI" },
      { label: "A/D", key: "AD" },
      { label: "ChkOsc", key: "CHKOSC" },
      { label: "VWBand", key: "VWBAND" },
    ],
  },
  {
    title: "强度",
    tone: "#a855f7",
    items: [
      { label: "ADX", key: "ADX" },
      { label: "Pivot", key: "PIVOT" },
      { label: "CHOP", key: "CHOP" },
      { label: "Elder", key: "ELDER" },
      { label: "Ulcer", key: "ULCER" },
      { label: "信号比", key: "SIGNALRATIO" },
    ],
  },
];

export const goStockIndicatorKeys: IndicatorKey[] = goStockIndicatorGroups.flatMap((group) =>
  group.items.map((item) => item.key),
);

export const goStockCandleIndicators: IndicatorKey[] = [
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
];

export const goStockPaneIndicators: IndicatorKey[] = goStockIndicatorKeys.filter(
  (key) => !goStockCandleIndicators.includes(key),
);

let registered = false;

export function registerGoStockIndicators() {
  if (registered) {
    return;
  }
  for (const indicator of goStockIndicatorTemplates) {
    registerIndicator(indicator);
  }
  registered = true;
}

const line = (key: string, title: string): IndicatorFigure<IndicatorRow> => ({ key, title, type: "line" });
const bar = (key: string, title: string, signalKey = key): IndicatorFigure<IndicatorRow> => ({
  key,
  title,
  type: "bar",
  baseValue: 0,
  styles: ({ data }) => resolveIndicatorBarStyle(numericSignal(data.current?.[signalKey])),
});
const circle = (key: string, title: string): IndicatorFigure<IndicatorRow> => ({ key, title, type: "circle" });

const goStockIndicatorTemplates: Array<IndicatorTemplate<IndicatorRow, number>> = [
  seriesIndicator("MA", "MA", "price", [5, 10, 20, 60], [
    line("ma1", "MA5: "),
    line("ma2", "MA10: "),
    line("ma3", "MA20: "),
    line("ma4", "MA60: "),
  ], (data, params) => multiMaRows(closes(data), params, "ma")),
  seriesIndicator("EMA", "EMA", "price", [5, 10, 20, 60], [
    line("ema1", "EMA5: "),
    line("ema2", "EMA10: "),
    line("ema3", "EMA20: "),
    line("ema4", "EMA60: "),
  ], (data, params) => multiEmaRows(closes(data), params, "ema")),
  seriesIndicator("BOLL", "BOLL", "price", [20, 2], [
    line("up", "UP: "),
    line("mid", "MID: "),
    line("dn", "DN: "),
  ], (data, params) => {
    const boll = bollingerBands(closes(data), params[0] ?? 20, params[1] ?? 2);
    return rows({ up: boll.upper, mid: boll.mid, dn: boll.lower });
  }),
  seriesIndicator("SAR", "SAR", "price", [2, 20], [circle("sar", "SAR: ")], (data, params) => {
    const step = (params[0] ?? 2) / 100;
    const maxStep = (params[1] ?? 20) / 100;
    return rows({ sar: sarValues(highs(data), lows(data), closes(data), step, maxStep).sar });
  }),
  seriesIndicator("KAMA", "KAMA", "price", [10, 2, 30], [line("kama", "KAMA: ")], (data, params) =>
    rows({ kama: kamaValues(closes(data), params[0] ?? 10, params[1] ?? 2, params[2] ?? 30) }),
  ),
  seriesIndicator("STREND", "STrend", "price", [10, 3], [line("supertrend", "ST: ")], (data, params) =>
    rows({ supertrend: supertrendValues(highs(data), lows(data), closes(data), params[0] ?? 10, params[1] ?? 3).supertrend }),
  ),
  seriesIndicator("ICHI", "Ichi", "price", [9, 26, 52], [
    line("tenkan", "Tenkan: "),
    line("kijun", "Kijun: "),
    line("spanA", "SpanA: "),
    line("senkouB", "SpanB: "),
    line("chikou", "Chikou: "),
  ], (data, params) => {
    const ichi = ichimokuValues(highs(data), lows(data), closes(data), params[0] ?? 9, params[1] ?? 26, params[2] ?? 52);
    return rows(ichi);
  }),
  seriesIndicator("DEMA", "DEMA", "price", [21], [line("dema", "DEMA: ")], (data, params) =>
    rows({ dema: demaValues(closes(data), params[0] ?? 21) }),
  ),
  seriesIndicator("SATS", "SATS", "price", [], [
    line("stLine", "SATS: "),
    line("upper", "UP: "),
    line("lower", "DN: "),
  ], (data) => rows(satsValues(highs(data), lows(data), closes(data), volumes(data)))),
  seriesIndicator("GATOR", "Gator", "price", [13, 8, 5], [
    line("jaw", "Jaw: "),
    line("teeth", "Teeth: "),
    line("lips", "Lips: "),
  ], (data, params) => rows(alligatorValues(highs(data), lows(data), closes(data), params[0] ?? 13, params[1] ?? 8, params[2] ?? 5))),
  seriesIndicator("HULL", "Hull", "price", [9], [line("hull", "Hull: ")], (data, params) =>
    rows({ hull: hullMaValues(closes(data), params[0] ?? 9) }),
  ),
  seriesIndicator("TEMA", "TEMA", "price", [21], [line("tema", "TEMA: ")], (data, params) =>
    rows({ tema: temaValues(closes(data), params[0] ?? 21) }),
  ),
  seriesIndicator("KELT", "Kelt", "price", [20, 10, 15], [
    line("upper", "UP: "),
    line("mid", "MID: "),
    line("lower", "DN: "),
  ], (data, params) => rows(keltnerChannelValues(highs(data), lows(data), closes(data), params[0] ?? 20, params[1] ?? 10, (params[2] ?? 15) / 10))),
  seriesIndicator("DONCH", "Donch", "price", [20], [
    line("upper", "UP: "),
    line("mid", "MID: "),
    line("lower", "DN: "),
  ], (data, params) => rows(donchianChannelValues(highs(data), lows(data), params[0] ?? 20))),
  seriesIndicator("ZIGZAG", "ZigZag", "price", [5], [circle("zigzag", "ZZ: ")], (data, params) =>
    rows({ zigzag: zigzagValues(highs(data), lows(data), closes(data), params[0] ?? 5).zigzag }),
  ),
  seriesIndicator("FRACTAL", "Fractal", "price", [2], [
    circle("fractalHigh", "FH: "),
    circle("fractalLow", "FL: "),
  ], (data, params) => rows(fractalValues(highs(data), lows(data), params[0] ?? 2))),
  seriesIndicator("SMC", "SMC", "price", [5, 50], [
    circle("swingHigh", "SH: "),
    circle("swingLow", "SL: "),
    circle("intHigh", "IH: "),
    circle("intLow", "IL: "),
  ], (data, params) => {
    const smc = smcValues(highs(data), lows(data), closes(data), opens(data), params[0] ?? 5, params[1] ?? 50);
    return rows({ swingHigh: smc.swingHighs, swingLow: smc.swingLows, intHigh: smc.intHighs, intLow: smc.intLows });
  }),
  seriesIndicator("VWAP", "VWAP", "price", [20], [line("vwap", "VWAP: ")], (data, params) =>
    rows({ vwap: vwapValues(highs(data), lows(data), closes(data), volumes(data), params[0] ?? 20) }),
  ),
  seriesIndicator("VWBAND", "VWBand", "price", [20, 2], [
    line("vwap", "VWAP: "),
    line("upper", "UP: "),
    line("lower", "DN: "),
  ], (data, params) => rows(vwapBandsValues(highs(data), lows(data), closes(data), volumes(data), params[0] ?? 20, params[1] ?? 2))),
  seriesIndicator("PIVOT", "Pivot", "price", [], [
    line("pp", "PP: "),
    line("r1", "R1: "),
    line("s1", "S1: "),
    line("r2", "R2: "),
    line("s2", "S2: "),
  ], (data) => rows(pivotPointsValues(highs(data), lows(data), closes(data)))),
  seriesIndicator("VOL", "VOL", "volume", [5, 10], [
    line("ma1", "MA5: "),
    line("ma2", "MA10: "),
    bar("volume", "VOLUME: ", "volumeColorSignal"),
  ], (data, params) => {
    const vol = volumes(data);
    return rows({
      ma1: smaValues(vol, params[0] ?? 5),
      ma2: smaValues(vol, params[1] ?? 10),
      volume: vol,
      volumeColorSignal: volumeColorSignals(data),
    });
  }, true),
  seriesIndicator("MACD", "MACD", "normal", [], [
    line("dif", "DIF: "),
    line("dea", "DEA: "),
    bar("macd", "MACD: "),
  ], (data) => {
    const macd = macdBundle(closes(data));
    return rows({ dif: macd.dif, dea: macd.dea, macd: macd.hist });
  }),
  seriesIndicator("KDJ", "KDJ", "normal", [9], [line("k", "K: "), line("d", "D: "), line("j", "J: ")], (data, params) => {
    const kdj = kdjBundle(highs(data), lows(data), closes(data), params[0] ?? 9);
    return rows({ k: kdj.K, d: kdj.D, j: kdj.J });
  }),
  seriesIndicator("RSI", "RSI", "normal", [6, 12, 24], [
    line("rsi1", "RSI6: "),
    line("rsi2", "RSI12: "),
    line("rsi3", "RSI24: "),
  ], (data, params) => rows({ rsi1: rsiBundle(closes(data), params[0] ?? 6), rsi2: rsiBundle(closes(data), params[1] ?? 12), rsi3: rsiBundle(closes(data), params[2] ?? 24) })),
  seriesIndicator("WR", "WR", "normal", [14], [line("wr", "WR: ")], (data, params) =>
    rows({ wr: williamsRValues(highs(data), lows(data), closes(data), params[0] ?? 14) }),
  ),
  seriesIndicator("OBV", "OBV", "normal", [30], [line("obv", "OBV: "), line("maObv", "MAOBV: ")], (data, params) => {
    const obv = obvValues(closes(data), volumes(data));
    return rows({ obv, maObv: smaValues(obv, params[0] ?? 30) });
  }, true),
  seriesIndicator("CCI", "CCI", "normal", [20], [line("cci", "CCI: ")], (data, params) =>
    rows({ cci: cciValues(highs(data), lows(data), closes(data), params[0] ?? 20) }),
  ),
  seriesIndicator("AO", "AO", "normal", [5, 34], [bar("ao", "AO: ")], (data, params) =>
    rows({ ao: aoValues(highs(data), lows(data), params[0] ?? 5, params[1] ?? 34) }),
  ),
  seriesIndicator("TRIX", "TRIX", "normal", [15], [line("trix", "TRIX: "), line("slope", "Slope: ")], (data, params) =>
    rows({ trix: trixValues(closes(data), params[0] ?? 15), slope: trixSlopeValues(closes(data), params[0] ?? 15) }),
  ),
  seriesIndicator("ROC", "ROC", "normal", [12], [line("roc", "ROC: ")], (data, params) =>
    rows({ roc: rocValues(closes(data), params[0] ?? 12) }),
  ),
  seriesIndicator("PVT", "PVT", "normal", [], [line("pvt", "PVT: ")], (data) =>
    rows({ pvt: pvtValues(closes(data), volumes(data)) }),
    true,
  ),
  seriesIndicator("ADX", "ADX", "normal", [14], [
    line("adx", "ADX: "),
    line("diP", "+DI: "),
    line("diM", "-DI: "),
  ], (data, params) => rows(adxValues(highs(data), lows(data), closes(data), params[0] ?? 14))),
  seriesIndicator("ATR", "ATR", "normal", [14], [line("atr", "ATR: ")], (data, params) =>
    rows({ atr: atrValues(highs(data), lows(data), closes(data), params[0] ?? 14) }),
  ),
  seriesIndicator("AVGAMP", "均幅", "normal", [5, 10, 20], [
    line("amp5", "AMP5: "),
    line("amp10", "AMP10: "),
    line("amp20", "AMP20: "),
  ], (data, params) => {
    const amp = data.map((bar) => (bar.close !== 0 ? ((bar.high - bar.low) / bar.close) * 100 : null));
    return rows({ amp5: smaValues(amp, params[0] ?? 5), amp10: smaValues(amp, params[1] ?? 10), amp20: smaValues(amp, params[2] ?? 20) });
  }),
  seriesIndicator("TTM", "TTM", "normal", [], [bar("momentum", "MOM: ")], (data) =>
    rows({ momentum: ttmSqueezeValues(highs(data), lows(data), closes(data)).momentum }),
  ),
  seriesIndicator("MASS", "Mass", "normal", [], [line("mass", "Mass: ")], (data) =>
    rows({ mass: massIndexValues(highs(data), lows(data)) }),
  ),
  seriesIndicator("SRSI", "SRSI", "normal", [], [line("k", "K: "), line("d", "D: ")], (data) =>
    rows(stochRsiValues(closes(data))),
  ),
  seriesIndicator("CMO", "CMO", "normal", [14], [line("cmo", "CMO: ")], (data, params) =>
    rows({ cmo: cmoValues(closes(data), params[0] ?? 14) }),
  ),
  seriesIndicator("SMI", "SMI", "normal", [], [line("smi", "SMI: "), line("signal", "Signal: ")], (data) =>
    rows(smiValues(highs(data), lows(data), closes(data))),
  ),
  seriesIndicator("COPPCK", "Coppck", "normal", [], [line("coppock", "Coppock: ")], (data) =>
    rows({ coppock: coppockValues(closes(data)) }),
  ),
  seriesIndicator("MFI", "MFI", "normal", [14], [line("mfi", "MFI: ")], (data, params) =>
    rows({ mfi: mfiValues(highs(data), lows(data), closes(data), volumes(data), params[0] ?? 14) }),
  ),
  seriesIndicator("CMF", "CMF", "normal", [20], [line("cmf", "CMF: ")], (data, params) =>
    rows({ cmf: cmfValues(highs(data), lows(data), closes(data), volumes(data), params[0] ?? 20) }),
  ),
  seriesIndicator("FI", "FI", "normal", [13], [line("fi", "FI: ")], (data, params) =>
    rows({ fi: forceIndexValues(closes(data), volumes(data), params[0] ?? 13) }),
    true,
  ),
  seriesIndicator("AD", "A/D", "normal", [], [line("ad", "A/D: ")], (data) =>
    rows({ ad: adValues(highs(data), lows(data), closes(data), volumes(data)) }),
    true,
  ),
  seriesIndicator("CHKOSC", "ChkOsc", "normal", [], [line("chkosc", "ChkOsc: ")], (data) =>
    rows({ chkosc: chaikinOscValues(highs(data), lows(data), closes(data), volumes(data)) }),
    true,
  ),
  seriesIndicator("AROON", "Aroon", "normal", [25], [line("up", "Up: "), line("down", "Down: ")], (data, params) =>
    rows(aroonValues(highs(data), lows(data), params[0] ?? 25)),
  ),
  seriesIndicator("CHOP", "CHOP", "normal", [14], [line("chop", "CHOP: ")], (data, params) =>
    rows({ chop: chopValues(highs(data), lows(data), closes(data), params[0] ?? 14) }),
  ),
  seriesIndicator("ELDER", "Elder", "normal", [13], [line("bullPower", "Bull: "), line("bearPower", "Bear: ")], (data, params) =>
    rows(elderRayValues(highs(data), lows(data), closes(data), params[0] ?? 13)),
  ),
  seriesIndicator("ULCER", "Ulcer", "normal", [14], [line("ulcer", "Ulcer: ")], (data, params) =>
    rows({ ulcer: ulcerIndexValues(closes(data), params[0] ?? 14) }),
  ),
  seriesIndicator("SIGNALRATIO", "信号比", "normal", [], [
    line("bull", "Bull%: "),
    line("bear", "Bear%: "),
    line("net", "Net%: "),
  ], (data) => rows(signalRatioValues(highs(data), lows(data), closes(data), volumes(data)))),
];

function seriesIndicator(
  name: string,
  shortName: string,
  series: "normal" | "price" | "volume",
  calcParams: number[],
  figures: Array<IndicatorFigure<IndicatorRow>>,
  calc: (dataList: KLineData[], calcParams: number[]) => IndicatorRow[],
  shouldFormatBigNumber = false,
): IndicatorTemplate<IndicatorRow, number> {
  return {
    name,
    shortName,
    series,
    precision: shouldFormatBigNumber ? 0 : 4,
    calcParams,
    figures,
    shouldFormatBigNumber,
    calc: (dataList, indicator) => calc(dataList, indicator.calcParams),
  };
}

export function resolveIndicatorBarStyle(signal: number): IndicatorFigureStyle {
  const color = signal > 0 ? indicatorUpColor : signal < 0 ? indicatorDownColor : indicatorNeutralColor;
  return { color, borderColor: color, style: "fill" } as IndicatorFigureStyle;
}

function numericSignal(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function rows(series: Record<string, Array<NullableNumber | boolean>>): IndicatorRow[] {
  const length = Math.max(0, ...Object.values(series).map((values) => values.length));
  return Array.from({ length }, (_, index) => {
    const row: IndicatorRow = {};
    for (const [key, values] of Object.entries(series)) {
      const value = values[index];
      if (value !== null && value !== undefined) {
        row[key] = value;
      }
    }
    return row;
  });
}

function opens(data: KLineData[]) {
  return data.map((bar) => bar.open);
}

function highs(data: KLineData[]) {
  return data.map((bar) => bar.high);
}

function lows(data: KLineData[]) {
  return data.map((bar) => bar.low);
}

function closes(data: KLineData[]) {
  return data.map((bar) => bar.close);
}

function volumes(data: KLineData[]) {
  return data.map((bar) => (Number.isFinite(bar.volume) ? bar.volume ?? 0 : 0));
}

function volumeColorSignals(data: KLineData[]) {
  return data.map((bar, index) => {
    if (bar.close > bar.open) {
      return 1;
    }
    if (bar.close < bar.open) {
      return -1;
    }
    const previousClose = index > 0 ? data[index - 1]?.close : undefined;
    if (typeof previousClose === "number") {
      if (bar.close > previousClose) {
        return 1;
      }
      if (bar.close < previousClose) {
        return -1;
      }
    }
    return 0;
  });
}

function smaValues(values: Array<NullableNumber>, period: number): NullableNumber[] {
  const out: NullableNumber[] = [];
  for (let i = 0; i < values.length; i += 1) {
    if (i < period - 1) {
      out.push(null);
      continue;
    }
    let sum = 0;
    let ok = true;
    for (let j = 0; j < period; j += 1) {
      const value = values[i - j];
      if (!isFiniteNumber(value)) {
        ok = false;
        break;
      }
      sum += value;
    }
    out.push(ok ? sum / period : null);
  }
  return out;
}

function emaFinite(values: Array<NullableNumber>, period: number): NullableNumber[] {
  const out: NullableNumber[] = [];
  const factor = 2 / (period + 1);
  let ema: number | null = null;
  for (let i = 0; i < values.length; i += 1) {
    const value = values[i];
    if (!isFiniteNumber(value)) {
      out.push(null);
      continue;
    }
    if (ema === null) {
      if (i < period - 1) {
        out.push(null);
        continue;
      }
      let sum = 0;
      let ok = true;
      for (let j = i - period + 1; j <= i; j += 1) {
        const windowValue = values[j];
        if (!isFiniteNumber(windowValue)) {
          ok = false;
          break;
        }
        sum += windowValue;
      }
      if (!ok) {
        out.push(null);
        continue;
      }
      ema = sum / period;
      out.push(ema);
    } else {
      ema = value * factor + ema * (1 - factor);
      out.push(ema);
    }
  }
  return out;
}

function emaLeadingNull(series: Array<NullableNumber>, period: number): NullableNumber[] {
  const out = series.map(() => null as NullableNumber);
  const factor = 2 / (period + 1);
  let ema: number | null = null;
  let sum = 0;
  let count = 0;
  for (let i = 0; i < series.length; i += 1) {
    const value = series[i];
    if (!isFiniteNumber(value)) {
      out[i] = null;
      continue;
    }
    if (ema === null) {
      sum += value;
      count += 1;
      if (count < period) {
        out[i] = null;
        continue;
      }
      ema = sum / period;
      out[i] = ema;
    } else {
      ema = value * factor + ema * (1 - factor);
      out[i] = ema;
    }
  }
  return out;
}

function weightedMaValues(values: Array<NullableNumber>, period: number): NullableNumber[] {
  const out: NullableNumber[] = [];
  const denom = (period * (period + 1)) / 2;
  for (let i = 0; i < values.length; i += 1) {
    if (i < period - 1) {
      out.push(null);
      continue;
    }
    let sum = 0;
    let ok = true;
    for (let j = 0; j < period; j += 1) {
      const value = values[i - period + 1 + j];
      if (!isFiniteNumber(value)) {
        ok = false;
        break;
      }
      sum += value * (j + 1);
    }
    out.push(ok ? sum / denom : null);
  }
  return out;
}

function multiMaRows(values: number[], params: number[], prefix: string) {
  return rows(Object.fromEntries(params.map((period, index) => [`${prefix}${index + 1}`, smaValues(values, period)])));
}

function multiEmaRows(values: number[], params: number[], prefix: string) {
  return rows(Object.fromEntries(params.map((period, index) => [`${prefix}${index + 1}`, emaFinite(values, period)])));
}

function bollingerBands(values: number[], period: number, mult: number) {
  const mid = smaValues(values, period);
  const upper: NullableNumber[] = [];
  const lower: NullableNumber[] = [];
  for (let i = 0; i < values.length; i += 1) {
    const midValue = mid[i];
    if (i < period - 1 || !isFiniteNumber(midValue)) {
      upper.push(null);
      lower.push(null);
      continue;
    }
    let sumSq = 0;
    for (let j = 0; j < period; j += 1) {
      const diff = values[i - j] - midValue;
      sumSq += diff * diff;
    }
    const std = Math.sqrt(sumSq / period);
    upper.push(midValue + mult * std);
    lower.push(midValue - mult * std);
  }
  return { upper, mid, lower };
}

function obvValues(close: number[], vol: number[]) {
  if (!close.length) {
    return [];
  }
  const out: number[] = [];
  let obv = vol[0] || 0;
  out.push(obv);
  for (let i = 1; i < close.length; i += 1) {
    const change = close[i] - close[i - 1];
    if (change > 0) {
      obv += vol[i] || 0;
    } else if (change < 0) {
      obv -= vol[i] || 0;
    }
    out.push(obv);
  }
  return out;
}

function macdBundle(close: number[]) {
  const ema12 = emaFinite(close, 12);
  const ema26 = emaFinite(close, 26);
  const dif = close.map((_, index) => (ema12[index] !== null && ema26[index] !== null ? ema12[index] - ema26[index] : null));
  const dea = emaLeadingNull(dif, 9);
  const hist = dif.map((value, index) => (value !== null && dea[index] !== null ? 2 * (value - dea[index]) : null));
  return { dif, dea, hist };
}

function kdjBundle(high: number[], low: number[], close: number[], period = 9) {
  const length = close.length;
  const rsv: NullableNumber[] = new Array(length).fill(null);
  for (let i = period - 1; i < length; i += 1) {
    let highest = -Infinity;
    let lowest = Infinity;
    for (let j = 0; j < period; j += 1) {
      highest = Math.max(highest, high[i - j]);
      lowest = Math.min(lowest, low[i - j]);
    }
    rsv[i] = highest === lowest ? 50 : ((close[i] - lowest) / (highest - lowest)) * 100;
  }
  const K: NullableNumber[] = new Array(length).fill(null);
  const D: NullableNumber[] = new Array(length).fill(null);
  const J: NullableNumber[] = new Array(length).fill(null);
  let prevK = 50;
  let prevD = 50;
  for (let i = 0; i < length; i += 1) {
    const value = rsv[i];
    if (value === null) {
      continue;
    }
    prevK = (2 * prevK + value) / 3;
    prevD = (2 * prevD + prevK) / 3;
    K[i] = prevK;
    D[i] = prevD;
    J[i] = 3 * prevK - 2 * prevD;
  }
  return { K, D, J };
}

function rsiBundle(close: number[], period = 14) {
  const out: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = period; i < close.length; i += 1) {
    let gain = 0;
    let loss = 0;
    for (let j = 0; j < period; j += 1) {
      const change = close[i - j] - close[i - j - 1];
      if (change >= 0) {
        gain += change;
      } else {
        loss -= change;
      }
    }
    const avgGain = gain / period;
    const avgLoss = loss / period;
    out[i] = avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss);
  }
  return out;
}

function atrValues(high: number[], low: number[], close: number[], period = 14) {
  const length = close.length;
  if (length < 2) {
    return new Array(length).fill(null) as NullableNumber[];
  }
  const tr: number[] = new Array(length).fill(0);
  tr[0] = high[0] - low[0];
  for (let i = 1; i < length; i += 1) {
    tr[i] = Math.max(high[i] - low[i], Math.abs(high[i] - close[i - 1]), Math.abs(low[i] - close[i - 1]));
  }
  const out: NullableNumber[] = new Array(length).fill(null);
  let sum = 0;
  for (let i = 0; i < period && i < length; i += 1) {
    sum += tr[i];
  }
  if (length >= period) {
    out[period - 1] = sum / period;
    for (let i = period; i < length; i += 1) {
      out[i] = ((out[i - 1] ?? 0) * (period - 1) + tr[i]) / period;
    }
  }
  return out;
}

function vwapValues(high: number[], low: number[], close: number[], vol: number[], period = 20) {
  const length = close.length;
  const out: NullableNumber[] = new Array(length).fill(null);
  for (let i = period - 1; i < length; i += 1) {
    let sumPV = 0;
    let sumV = 0;
    for (let j = 0; j < period; j += 1) {
      const typical = (high[i - j] + low[i - j] + close[i - j]) / 3;
      sumPV += typical * vol[i - j];
      sumV += vol[i - j];
    }
    out[i] = sumV > 0 ? sumPV / sumV : null;
  }
  return out;
}

function mfiValues(high: number[], low: number[], close: number[], vol: number[], period = 14) {
  const length = close.length;
  if (length < 2) {
    return new Array(length).fill(null) as NullableNumber[];
  }
  const typical = close.map((_, index) => (high[index] + low[index] + close[index]) / 3);
  const moneyFlow = typical.map((value, index) => value * vol[index]);
  const out: NullableNumber[] = new Array(length).fill(null);
  for (let i = period; i < length; i += 1) {
    let positive = 0;
    let negative = 0;
    for (let j = 0; j < period; j += 1) {
      const index = i - j;
      if (typical[index] > typical[index - 1]) {
        positive += moneyFlow[index];
      } else if (typical[index] < typical[index - 1]) {
        negative += moneyFlow[index];
      }
    }
    out[i] = negative === 0 ? 100 : 100 - 100 / (1 + positive / negative);
  }
  return out;
}

function kamaValues(close: number[], period = 10, fastPeriod = 2, slowPeriod = 30) {
  const length = close.length;
  const out: NullableNumber[] = new Array(length).fill(null);
  if (length < period + 1) {
    return out;
  }
  const fastSC = 2 / (fastPeriod + 1);
  const slowSC = 2 / (slowPeriod + 1);
  let kama = close[period];
  out[period] = kama;
  for (let i = period + 1; i < length; i += 1) {
    const direction = Math.abs(close[i] - close[i - period]);
    let volatility = 0;
    for (let j = 0; j < period; j += 1) {
      volatility += Math.abs(close[i - j] - close[i - j - 1]);
    }
    const er = volatility > 0 ? direction / volatility : 0;
    const sc = (er * (fastSC - slowSC) + slowSC) ** 2;
    kama = kama + sc * (close[i] - kama);
    out[i] = kama;
  }
  return out;
}

function keltnerChannelValues(high: number[], low: number[], close: number[], emaPeriod = 20, atrPeriod = 10, mult = 1.5) {
  const mid = emaFinite(close, emaPeriod);
  const atr = atrValues(high, low, close, atrPeriod);
  const upper: NullableNumber[] = [];
  const lower: NullableNumber[] = [];
  for (let i = 0; i < close.length; i += 1) {
    const midValue = mid[i];
    const atrValue = atr[i];
    if (midValue !== null && atrValue !== null) {
      upper.push(midValue + mult * atrValue);
      lower.push(midValue - mult * atrValue);
    } else {
      upper.push(null);
      lower.push(null);
    }
  }
  return { upper, mid, lower };
}

function supertrendValues(high: number[], low: number[], close: number[], atrPeriod = 10, multiplier = 3) {
  const length = close.length;
  const atr = atrValues(high, low, close, atrPeriod);
  const supertrend: NullableNumber[] = new Array(length).fill(null);
  const direction: number[] = new Array(length).fill(0);
  let prevUpper: number | null = null;
  let prevLower: number | null = null;
  let prevDir = 0;
  for (let i = 0; i < length; i += 1) {
    const atrValue = atr[i];
    if (atrValue === null) {
      continue;
    }
    const hl2 = (high[i] + low[i]) / 2;
    let rawUpper = hl2 + multiplier * atrValue;
    let rawLower = hl2 - multiplier * atrValue;
    if (prevUpper !== null && rawUpper >= prevUpper && close[i - 1] <= prevUpper) {
      rawUpper = prevUpper;
    }
    if (prevLower !== null && rawLower <= prevLower && close[i - 1] >= prevLower) {
      rawLower = prevLower;
    }
    let dir: number;
    if (prevDir === 0) {
      dir = 1;
    } else if (prevDir === 1) {
      dir = close[i] < rawLower ? -1 : 1;
    } else {
      dir = close[i] > rawUpper ? 1 : -1;
    }
    supertrend[i] = dir === 1 ? rawLower : rawUpper;
    direction[i] = dir;
    prevUpper = rawUpper;
    prevLower = rawLower;
    prevDir = dir;
  }
  return { supertrend, direction };
}

function ichimokuValues(high: number[], low: number[], close: number[], tenkanP = 9, kijunP = 26, senkouBP = 52) {
  const length = close.length;
  const periodHL = (period: number) => {
    const out: NullableNumber[] = new Array(length).fill(null);
    for (let i = period - 1; i < length; i += 1) {
      let hi = -Infinity;
      let lo = Infinity;
      for (let j = 0; j < period; j += 1) {
        hi = Math.max(hi, high[i - j]);
        lo = Math.min(lo, low[i - j]);
      }
      out[i] = (hi + lo) / 2;
    }
    return out;
  };
  const tenkan = periodHL(tenkanP);
  const kijun = periodHL(kijunP);
  const senkouB = periodHL(senkouBP);
  const spanA: NullableNumber[] = new Array(length).fill(null);
  const chikou: NullableNumber[] = new Array(length).fill(null);
  for (let i = 0; i < length; i += 1) {
    const tenkanValue = tenkan[i];
    const kijunValue = kijun[i];
    if (tenkanValue !== null && kijunValue !== null) {
      spanA[i] = (tenkanValue + kijunValue) / 2;
    }
    if (i + kijunP < length) {
      chikou[i] = close[i + kijunP];
    }
  }
  return { tenkan, kijun, spanA, senkouB, chikou };
}

function cciValues(high: number[], low: number[], close: number[], period = 20) {
  const typical = close.map((_, index) => (high[index] + low[index] + close[index]) / 3);
  const out: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = period - 1; i < close.length; i += 1) {
    let sum = 0;
    for (let j = 0; j < period; j += 1) {
      sum += typical[i - j];
    }
    const mean = sum / period;
    let meanDev = 0;
    for (let j = 0; j < period; j += 1) {
      meanDev += Math.abs(typical[i - j] - mean);
    }
    meanDev /= period;
    out[i] = meanDev > 0 ? (typical[i] - mean) / (0.015 * meanDev) : null;
  }
  return out;
}

function ttmSqueezeValues(high: number[], low: number[], close: number[]) {
  const typical = close.map((_, index) => (high[index] + low[index] + close[index]) / 3);
  const emaTypical = emaFinite(typical, 20);
  const momentum = typical.map((value, index) => (emaTypical[index] !== null ? value - emaTypical[index] : null));
  return { momentum };
}

function sarValues(high: number[], low: number[], close: number[], step = 0.02, maxStep = 0.2) {
  const length = close.length;
  if (length < 2) {
    return { sar: new Array(length).fill(null) as NullableNumber[], direction: new Array(length).fill(0) };
  }
  const sar: NullableNumber[] = new Array(length).fill(null);
  const direction = new Array(length).fill(0);
  let isLong = close[1] > close[0];
  let af = step;
  let ep = isLong ? high[1] : low[1];
  let prevSar = isLong ? low[0] : high[0];
  sar[1] = prevSar;
  direction[1] = isLong ? 1 : -1;
  for (let i = 2; i < length; i += 1) {
    let currentSar = prevSar + af * (ep - prevSar);
    if (isLong) {
      currentSar = Math.min(currentSar, low[i - 1], low[i - 2]);
      if (low[i] < currentSar) {
        isLong = false;
        currentSar = ep;
        ep = low[i];
        af = step;
      } else if (high[i] > ep) {
        ep = high[i];
        af = Math.min(af + step, maxStep);
      }
    } else {
      currentSar = Math.max(currentSar, high[i - 1], high[i - 2]);
      if (high[i] > currentSar) {
        isLong = true;
        currentSar = ep;
        ep = high[i];
        af = step;
      } else if (low[i] < ep) {
        ep = low[i];
        af = Math.min(af + step, maxStep);
      }
    }
    sar[i] = currentSar;
    direction[i] = isLong ? 1 : -1;
    prevSar = currentSar;
  }
  return { sar, direction };
}

function donchianChannelValues(high: number[], low: number[], period = 20) {
  const length = high.length;
  const upper: NullableNumber[] = new Array(length).fill(null);
  const lower: NullableNumber[] = new Array(length).fill(null);
  const mid: NullableNumber[] = new Array(length).fill(null);
  for (let i = period - 1; i < length; i += 1) {
    let hi = -Infinity;
    let lo = Infinity;
    for (let j = 0; j < period; j += 1) {
      hi = Math.max(hi, high[i - j]);
      lo = Math.min(lo, low[i - j]);
    }
    upper[i] = hi;
    lower[i] = lo;
    mid[i] = (hi + lo) / 2;
  }
  return { upper, mid, lower };
}

function adxValues(high: number[], low: number[], close: number[], period = 14) {
  const length = close.length;
  if (length < 2) {
    return { adx: new Array(length).fill(null) as NullableNumber[], diP: new Array(length).fill(null) as NullableNumber[], diM: new Array(length).fill(null) as NullableNumber[] };
  }
  const tr = new Array(length).fill(0);
  const plusDM = new Array(length).fill(0);
  const minusDM = new Array(length).fill(0);
  tr[0] = high[0] - low[0];
  for (let i = 1; i < length; i += 1) {
    tr[i] = Math.max(high[i] - low[i], Math.abs(high[i] - close[i - 1]), Math.abs(low[i] - close[i - 1]));
    const upMove = high[i] - high[i - 1];
    const downMove = low[i - 1] - low[i];
    plusDM[i] = upMove > downMove && upMove > 0 ? upMove : 0;
    minusDM[i] = downMove > upMove && downMove > 0 ? downMove : 0;
  }
  const smooth = (values: number[]) => {
    const out: NullableNumber[] = new Array(length).fill(null);
    let sum = 0;
    for (let i = 0; i < period && i < length; i += 1) {
      sum += values[i];
    }
    if (length >= period) {
      out[period - 1] = sum;
      for (let i = period; i < length; i += 1) {
        out[i] = (out[i - 1] ?? 0) - (out[i - 1] ?? 0) / period + values[i];
      }
    }
    return out;
  };
  const smoothTR = smooth(tr);
  const smoothPDM = smooth(plusDM);
  const smoothMDM = smooth(minusDM);
  const diP: NullableNumber[] = new Array(length).fill(null);
  const diM: NullableNumber[] = new Array(length).fill(null);
  const dx: NullableNumber[] = new Array(length).fill(null);
  for (let i = 0; i < length; i += 1) {
    const trValue = smoothTR[i];
    const pdmValue = smoothPDM[i];
    const mdmValue = smoothMDM[i];
    if (trValue !== null && trValue > 0 && pdmValue !== null && mdmValue !== null) {
      const plus = (100 * pdmValue) / trValue;
      const minus = (100 * mdmValue) / trValue;
      diP[i] = plus;
      diM[i] = minus;
      const sum = plus + minus;
      dx[i] = sum > 0 ? (100 * Math.abs(plus - minus)) / sum : 0;
    }
  }
  const adx: NullableNumber[] = new Array(length).fill(null);
  if (length >= period * 2 - 1) {
    let sumDx = 0;
    for (let i = period - 1; i < period * 2 - 1 && i < length; i += 1) {
      sumDx += dx[i] || 0;
    }
    adx[period * 2 - 2] = sumDx / period;
    for (let i = period * 2 - 1; i < length; i += 1) {
      adx[i] = ((adx[i - 1] ?? 0) * (period - 1) + (dx[i] || 0)) / period;
    }
  }
  return { adx, diP, diM };
}

function williamsRValues(high: number[], low: number[], close: number[], period = 14) {
  const out: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = period - 1; i < close.length; i += 1) {
    let hi = -Infinity;
    let lo = Infinity;
    for (let j = 0; j < period; j += 1) {
      hi = Math.max(hi, high[i - j]);
      lo = Math.min(lo, low[i - j]);
    }
    const range = hi - lo;
    out[i] = range > 0 ? ((hi - close[i]) / range) * -100 : null;
  }
  return out;
}

function stochRsiValues(close: number[], rsiPeriod = 14, stochPeriod = 14, kSmooth = 3, dSmooth = 3) {
  const rsi = rsiBundle(close, rsiPeriod);
  const length = close.length;
  const stochRsi: NullableNumber[] = new Array(length).fill(null);
  for (let i = stochPeriod - 1; i < length; i += 1) {
    let minRsi = Infinity;
    let maxRsi = -Infinity;
    let valid = true;
    for (let j = 0; j < stochPeriod; j += 1) {
      const rsiValue = rsi[i - j];
      if (rsiValue === null) {
        valid = false;
        break;
      }
      minRsi = Math.min(minRsi, rsiValue);
      maxRsi = Math.max(maxRsi, rsiValue);
    }
    if (valid) {
      stochRsi[i] = maxRsi !== minRsi ? ((rsi[i]! - minRsi) / (maxRsi - minRsi)) * 100 : 0;
    }
  }
  const k = smaValues(stochRsi, kSmooth);
  const d = smaValues(k, dSmooth);
  return { k, d };
}

function cmfValues(high: number[], low: number[], close: number[], vol: number[], period = 20) {
  const out: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = period - 1; i < close.length; i += 1) {
    let sumMFV = 0;
    let sumVol = 0;
    for (let j = 0; j < period; j += 1) {
      const index = i - j;
      const range = high[index] - low[index];
      const mfv = range > 0 ? (((close[index] - low[index]) - (high[index] - close[index])) / range) * vol[index] : 0;
      sumMFV += mfv;
      sumVol += vol[index];
    }
    out[i] = sumVol > 0 ? sumMFV / sumVol : null;
  }
  return out;
}

function aroonValues(high: number[], low: number[], period = 25) {
  const up: NullableNumber[] = new Array(high.length).fill(null);
  const down: NullableNumber[] = new Array(high.length).fill(null);
  for (let i = period - 1; i < high.length; i += 1) {
    let highIdx = 0;
    let lowIdx = 0;
    for (let j = 1; j < period; j += 1) {
      if (high[i - j] > high[i - highIdx]) {
        highIdx = j;
      }
      if (low[i - j] < low[i - lowIdx]) {
        lowIdx = j;
      }
    }
    up[i] = ((period - 1 - highIdx) / (period - 1)) * 100;
    down[i] = ((period - 1 - lowIdx) / (period - 1)) * 100;
  }
  return { up, down };
}

function cmoValues(close: number[], period = 14) {
  const out: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = period; i < close.length; i += 1) {
    let sumUp = 0;
    let sumDown = 0;
    for (let j = 0; j < period; j += 1) {
      const diff = close[i - j] - close[i - j - 1];
      if (diff > 0) {
        sumUp += diff;
      } else {
        sumDown -= diff;
      }
    }
    out[i] = sumUp + sumDown > 0 ? ((sumUp - sumDown) / (sumUp + sumDown)) * 100 : 0;
  }
  return out;
}

function forceIndexValues(close: number[], vol: number[], period = 13) {
  const raw: NullableNumber[] = new Array(close.length).fill(null);
  raw[0] = 0;
  for (let i = 1; i < close.length; i += 1) {
    raw[i] = (close[i] - close[i - 1]) * vol[i];
  }
  return emaFinite(raw, period);
}

function pivotPointsValues(high: number[], low: number[], close: number[]) {
  const pp: NullableNumber[] = new Array(close.length).fill(null);
  const s1: NullableNumber[] = new Array(close.length).fill(null);
  const s2: NullableNumber[] = new Array(close.length).fill(null);
  const r1: NullableNumber[] = new Array(close.length).fill(null);
  const r2: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = 1; i < close.length; i += 1) {
    const p = (high[i - 1] + low[i - 1] + close[i - 1]) / 3;
    pp[i] = p;
    r1[i] = 2 * p - low[i - 1];
    s1[i] = 2 * p - high[i - 1];
    r2[i] = p + (high[i - 1] - low[i - 1]);
    s2[i] = p - (high[i - 1] - low[i - 1]);
  }
  return { pp, s1, s2, r1, r2 };
}

function demaValues(close: number[], period = 21) {
  const ema1 = emaFinite(close, period);
  const ema2 = emaFinite(ema1.map((value) => value ?? null), period);
  return close.map((_, index) => (ema1[index] !== null && ema2[index] !== null ? 2 * ema1[index] - ema2[index] : null));
}

function zigzagValues(high: number[], low: number[], close: number[], threshold = 5) {
  const length = close.length;
  if (length < 3) {
    return { zigzag: new Array(length).fill(null) as NullableNumber[], directions: new Array(length).fill(0) };
  }
  const points: Array<{ idx: number; price: number; isHigh: boolean }> = [{ idx: 0, price: high[0], isHigh: true }];
  let lastHigh = { idx: 0, price: high[0] };
  let lastLow = { idx: 0, price: low[0] };
  let lookingFor: "high" | "low" = "high";
  for (let i = 1; i < length; i += 1) {
    if (lookingFor === "high") {
      if (high[i] >= lastHigh.price) {
        lastHigh = { idx: i, price: high[i] };
        points[points.length - 1] = { idx: i, price: high[i], isHigh: true };
      } else if (lastHigh.price - low[i] >= (lastHigh.price * threshold) / 100) {
        points.push({ idx: lastHigh.idx, price: lastHigh.price, isHigh: true });
        lastLow = { idx: i, price: low[i] };
        lookingFor = "low";
      }
    } else if (low[i] <= lastLow.price) {
      lastLow = { idx: i, price: low[i] };
      points[points.length - 1] = { idx: i, price: low[i], isHigh: false };
    } else if (high[i] - lastLow.price >= (lastLow.price * threshold) / 100) {
      points.push({ idx: lastLow.idx, price: lastLow.price, isHigh: false });
      lastHigh = { idx: i, price: high[i] };
      lookingFor = "high";
    }
  }
  const zigzag: NullableNumber[] = new Array(length).fill(null);
  const directions = new Array(length).fill(0);
  for (const point of points) {
    zigzag[point.idx] = point.price;
    directions[point.idx] = point.isHigh ? 1 : -1;
  }
  return { zigzag, directions };
}

function satsValues(high: number[], low: number[], close: number[], vol: number[]) {
  const st = supertrendValues(high, low, close, 14, 2);
  const atr = atrValues(high, low, close, 14);
  const stLine = st.supertrend;
  const upper = stLine.map((value, index) => (value !== null && atr[index] !== null ? value + atr[index] : null));
  const lower = stLine.map((value, index) => (value !== null && atr[index] !== null ? value - atr[index] : null));
  const volMean = smaValues(vol, 20);
  const tqi = vol.map((value, index) => (volMean[index] ? Math.min(1, value / (volMean[index] * 2)) : 0));
  return { stLine, upper, lower, direction: st.direction, tqi };
}

function alligatorValues(high: number[], low: number[], close: number[], jawLen = 13, teethLen = 8, lipsLen = 5, jawOffset = 8, teethOffset = 5, lipsOffset = 3) {
  const length = close.length;
  const mid = high.map((value, index) => (value + low[index]) / 2);
  const jawRaw = smaValues(mid, jawLen);
  const teethRaw = smaValues(mid, teethLen);
  const lipsRaw = smaValues(mid, lipsLen);
  const jaw: NullableNumber[] = new Array(length).fill(null);
  const teeth: NullableNumber[] = new Array(length).fill(null);
  const lips: NullableNumber[] = new Array(length).fill(null);
  for (let i = jawOffset; i < length; i += 1) {
    jaw[i] = jawRaw[i - jawOffset];
  }
  for (let i = teethOffset; i < length; i += 1) {
    teeth[i] = teethRaw[i - teethOffset];
  }
  for (let i = lipsOffset; i < length; i += 1) {
    lips[i] = lipsRaw[i - lipsOffset];
  }
  return { jaw, teeth, lips };
}

function aoValues(high: number[], low: number[], fastLen = 5, slowLen = 34) {
  const mid = high.map((value, index) => (value + low[index]) / 2);
  const fast = smaValues(mid, fastLen);
  const slow = smaValues(mid, slowLen);
  return high.map((_, index) => (fast[index] !== null && slow[index] !== null ? fast[index] - slow[index] : null));
}

function hullMaValues(close: number[], period = 9) {
  const halfLen = Math.floor(period / 2);
  const sqrtLen = Math.floor(Math.sqrt(period));
  const half = weightedMaValues(close, halfLen);
  const full = weightedMaValues(close, period);
  const diff = close.map((_, index) => (half[index] !== null && full[index] !== null ? 2 * half[index] - full[index] : null));
  return weightedMaValues(diff, sqrtLen);
}

function adValues(high: number[], low: number[], close: number[], vol: number[]) {
  const ad: number[] = new Array(close.length).fill(0);
  for (let i = 0; i < close.length; i += 1) {
    const range = high[i] - low[i];
    const mfm = range > 0 ? ((close[i] - low[i]) - (high[i] - close[i])) / range : 0;
    ad[i] = (i > 0 ? ad[i - 1] : 0) + mfm * (vol[i] || 0);
  }
  return ad;
}

function trixValues(close: number[], period = 15) {
  const ema1 = emaFinite(close, period);
  const ema2 = emaFinite(ema1, period);
  const ema3 = emaFinite(ema2, period);
  const trix: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = 1; i < close.length; i += 1) {
    const current = ema3[i];
    const prev = ema3[i - 1];
    if (current !== null && prev !== null && prev !== 0) {
      trix[i] = ((current - prev) / prev) * 10000;
    }
  }
  return trix;
}

function trixSlopeValues(close: number[], period = 15) {
  const trix = trixValues(close, period);
  return trix.map((value, index) => {
    const prev = index > 0 ? trix[index - 1] : null;
    return value !== null && prev !== null ? value - prev : null;
  });
}

function rocValues(close: number[], period = 12) {
  const out: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = period; i < close.length; i += 1) {
    if (close[i - period] !== 0) {
      out[i] = ((close[i] - close[i - period]) / close[i - period]) * 100;
    }
  }
  return out;
}

function fractalValues(high: number[], low: number[], period = 2) {
  const fractalHigh: NullableNumber[] = new Array(high.length).fill(null);
  const fractalLow: NullableNumber[] = new Array(high.length).fill(null);
  for (let i = period; i < high.length - period; i += 1) {
    let isHigh = true;
    let isLow = true;
    for (let j = 1; j <= period; j += 1) {
      if (high[i] <= high[i - j] || high[i] <= high[i + j]) {
        isHigh = false;
      }
      if (low[i] >= low[i - j] || low[i] >= low[i + j]) {
        isLow = false;
      }
    }
    if (isHigh) {
      fractalHigh[i] = high[i];
    }
    if (isLow) {
      fractalLow[i] = low[i];
    }
  }
  return { fractalHigh, fractalLow };
}

function chopValues(high: number[], low: number[], close: number[], period = 14) {
  const out: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = period - 1; i < close.length; i += 1) {
    let atrSum = 0;
    for (let j = 0; j < period; j += 1) {
      const index = i - j;
      atrSum += index > 0 ? Math.max(high[index] - low[index], Math.abs(high[index] - close[index - 1]), Math.abs(low[index] - close[index - 1])) : high[index] - low[index];
    }
    let hi = -Infinity;
    let lo = Infinity;
    for (let j = i - period + 1; j <= i; j += 1) {
      hi = Math.max(hi, high[j]);
      lo = Math.min(lo, low[j]);
    }
    const trueRange = hi - lo;
    out[i] = trueRange > 0 ? (100 * Math.log(atrSum / trueRange)) / Math.log(period) : null;
  }
  return out;
}

function elderRayValues(high: number[], low: number[], close: number[], emaPeriod = 13) {
  const ema = emaFinite(close, emaPeriod);
  const bullPower: NullableNumber[] = new Array(close.length).fill(null);
  const bearPower: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = 0; i < close.length; i += 1) {
    const emaValue = ema[i];
    if (emaValue !== null) {
      bullPower[i] = high[i] - emaValue;
      bearPower[i] = low[i] - emaValue;
    }
  }
  return { bullPower, bearPower };
}

function chaikinOscValues(high: number[], low: number[], close: number[], vol: number[], fastPeriod = 3, slowPeriod = 10) {
  const ad = adValues(high, low, close, vol);
  const fast = emaFinite(ad, fastPeriod);
  const slow = emaFinite(ad, slowPeriod);
  return close.map((_, index) => {
    const fastValue = fast[index];
    const slowValue = slow[index];
    return fastValue !== null && slowValue !== null ? fastValue - slowValue : null;
  });
}

function vwapBandsValues(high: number[], low: number[], close: number[], vol: number[], period = 20, mult = 2) {
  const vwap = vwapValues(high, low, close, vol, period);
  const upper: NullableNumber[] = new Array(close.length).fill(null);
  const lower: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = 0; i < close.length; i += 1) {
    const vwapValue = vwap[i];
    if (vwapValue === null) {
      continue;
    }
    let sumSq = 0;
    let count = 0;
    const start = Math.max(0, i - period + 1);
    for (let j = start; j <= i; j += 1) {
      const typical = (high[j] + low[j] + close[j]) / 3;
      const weight = vol[j] || 1;
      sumSq += (typical - vwapValue) ** 2 * weight;
      count += weight;
    }
    if (count > 0) {
      const std = Math.sqrt(sumSq / count);
      upper[i] = vwapValue + mult * std;
      lower[i] = vwapValue - mult * std;
    }
  }
  return { vwap, upper, lower };
}

function massIndexValues(high: number[], low: number[], emaPeriod = 9, emaPeriod2 = 9, sumPeriod = 25) {
  const range = high.map((value, index) => value - low[index]);
  const singleEma = emaFinite(range, emaPeriod);
  const doubleEma = emaFinite(singleEma, emaPeriod);
  const ratio = singleEma.map((value, index) => (value !== null && doubleEma[index] !== null && doubleEma[index] !== 0 ? value / doubleEma[index] : null));
  const ratioEma = emaFinite(ratio, emaPeriod2);
  const mass: NullableNumber[] = new Array(high.length).fill(null);
  for (let i = sumPeriod - 1; i < high.length; i += 1) {
    let sum = 0;
    let ok = true;
    for (let j = 0; j < sumPeriod; j += 1) {
      const ratioValue = ratioEma[i - j];
      if (ratioValue === null) {
        ok = false;
        break;
      }
      sum += ratioValue;
    }
    if (ok) {
      mass[i] = sum;
    }
  }
  return mass;
}

function ulcerIndexValues(close: number[], period = 14) {
  const out: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = period - 1; i < close.length; i += 1) {
    let maxClose = -Infinity;
    for (let j = 0; j < period; j += 1) {
      maxClose = Math.max(maxClose, close[i - j]);
    }
    let sumSq = 0;
    for (let j = 0; j < period; j += 1) {
      const drawdown = ((close[i - j] - maxClose) / maxClose) * 100;
      sumSq += drawdown * drawdown;
    }
    out[i] = Math.sqrt(sumSq / period);
  }
  return out;
}

function coppockValues(close: number[], wmaLen = 10, roc1 = 14, roc2 = 11) {
  const rocA = rocValues(close, roc1);
  const rocB = rocValues(close, roc2);
  const sum = close.map((_, index) => (rocA[index] !== null && rocB[index] !== null ? rocA[index] + rocB[index] : null));
  return weightedMaValues(sum, wmaLen);
}

function temaValues(close: number[], period = 21) {
  const ema1 = emaFinite(close, period);
  const ema2 = emaFinite(ema1, period);
  const ema3 = emaFinite(ema2, period);
  return close.map((_, index) => (ema1[index] !== null && ema2[index] !== null && ema3[index] !== null ? ema1[index] + (ema1[index] - ema2[index]) + (ema1[index] - ema2[index] - (ema2[index] - ema3[index])) : null));
}

function smiValues(high: number[], low: number[], close: number[], kPeriod = 14, dPeriod = 3, emaPeriod = 3) {
  const highest: NullableNumber[] = new Array(close.length).fill(null);
  const lowest: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = kPeriod - 1; i < close.length; i += 1) {
    let hi = -Infinity;
    let lo = Infinity;
    for (let j = 0; j < kPeriod; j += 1) {
      hi = Math.max(hi, high[i - j]);
      lo = Math.min(lo, low[i - j]);
    }
    highest[i] = hi;
    lowest[i] = lo;
  }
  const raw: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = 0; i < close.length; i += 1) {
    const highestValue = highest[i];
    const lowestValue = lowest[i];
    if (highestValue !== null && lowestValue !== null && highestValue !== lowestValue) {
      raw[i] = 200 * ((close[i] - (highestValue + lowestValue) / 2) / (highestValue - lowestValue));
    }
  }
  const smi = emaFinite(raw, emaPeriod);
  const signal = emaFinite(smi, dPeriod);
  return { smi, signal };
}

function smcValues(high: number[], low: number[], close: number[], open: number[], internalLen = 5, swingLen = 50) {
  void close;
  void open;
  const swingHighs: NullableNumber[] = new Array(high.length).fill(null);
  const swingLows: NullableNumber[] = new Array(high.length).fill(null);
  const intHighs: NullableNumber[] = new Array(high.length).fill(null);
  const intLows: NullableNumber[] = new Array(high.length).fill(null);
  markSwings(high, low, swingLen, swingHighs, swingLows);
  markSwings(high, low, internalLen, intHighs, intLows);
  return { swingHighs, swingLows, intHighs, intLows };
}

function markSwings(high: number[], low: number[], period: number, outHighs: NullableNumber[], outLows: NullableNumber[]) {
  for (let i = period; i < high.length - period; i += 1) {
    let isHigh = true;
    let isLow = true;
    for (let j = 1; j <= period; j += 1) {
      if (high[i] <= high[i - j] || high[i] <= high[i + j]) {
        isHigh = false;
      }
      if (low[i] >= low[i - j] || low[i] >= low[i + j]) {
        isLow = false;
      }
      if (!isHigh && !isLow) {
        break;
      }
    }
    if (isHigh) {
      outHighs[i] = high[i];
    }
    if (isLow) {
      outLows[i] = low[i];
    }
  }
}

function pvtValues(close: number[], vol: number[]) {
  const out: number[] = new Array(close.length).fill(0);
  for (let i = 1; i < close.length; i += 1) {
    out[i] = out[i - 1] + (close[i - 1] !== 0 ? ((close[i] - close[i - 1]) / close[i - 1]) * vol[i] : 0);
  }
  return out;
}

function signalRatioValues(high: number[], low: number[], close: number[], vol: number[]) {
  const macd = macdBundle(close);
  const kdj = kdjBundle(high, low, close);
  const rsi = rsiBundle(close, 14);
  const cci = cciValues(high, low, close);
  const obv = obvValues(close, vol);
  const st = supertrendValues(high, low, close);
  const bull: NullableNumber[] = new Array(close.length).fill(null);
  const bear: NullableNumber[] = new Array(close.length).fill(null);
  const net: NullableNumber[] = new Array(close.length).fill(null);
  for (let i = 1; i < close.length; i += 1) {
    const signals = [
      close[i] > close[i - 1] ? 1 : -1,
      compareNullable(macd.dif[i], macd.dea[i]),
      compareNullable(kdj.K[i], kdj.D[i]),
      compareThreshold(rsi[i], 50),
      compareThreshold(cci[i], 0),
      obv[i] >= obv[i - 1] ? 1 : -1,
      st.direction[i] >= 0 ? 1 : -1,
    ].filter((value) => value !== 0);
    const bullCount = signals.filter((value) => value > 0).length;
    const bearCount = signals.filter((value) => value < 0).length;
    if (signals.length > 0) {
      bull[i] = (bullCount / signals.length) * 100;
      bear[i] = (bearCount / signals.length) * 100;
      net[i] = (bull[i] ?? 0) - (bear[i] ?? 0);
    }
  }
  return { bull, bear, net };
}

function compareNullable(left: NullableNumber, right: NullableNumber) {
  return left !== null && right !== null ? (left >= right ? 1 : -1) : 0;
}

function compareThreshold(value: NullableNumber, threshold: number) {
  return value !== null ? (value >= threshold ? 1 : -1) : 0;
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}
