export type ChartPeriod = "minute" | "5m" | "15m" | "30m" | "60m" | "day" | "week" | "month" | "quarter" | "year";

export type AdjustType = "qfq" | "hfq" | "none";

export type IndicatorKey =
  | "MA"
  | "EMA"
  | "BOLL"
  | "SAR"
  | "MACD"
  | "KDJ"
  | "RSI"
  | "CCI"
  | "WR"
  | "AO"
  | "TRIX"
  | "ROC"
  | "VOL"
  | "OBV"
  | "PVT"
  | "DMI";

export type StockChartQuote = {
  name: string;
  code: string;
  symbol: string;
  price: number;
  changeAmount: number;
  changePercent: number;
  open: number;
  high: number;
  low: number;
  previousClose: number;
  volumeText: string;
  amountText: string;
  turnoverRateText: string;
  status: string;
  updateTime: string;
};

export type KlineBar = {
  date: string;
  open: number;
  close: number;
  low: number;
  high: number;
  volume: number;
  amount?: number;
};

export type IndicatorSeries = {
  ma5: Array<number | null>;
  ma10: Array<number | null>;
  ma20: Array<number | null>;
  ma60: Array<number | null>;
  bollUp: Array<number | null>;
  bollMid: Array<number | null>;
  bollDn: Array<number | null>;
  volMa5: Array<number | null>;
  volMa10: Array<number | null>;
  macdDif: Array<number | null>;
  macdDea: Array<number | null>;
  macdHist: Array<number | null>;
  kdjK: Array<number | null>;
  kdjD: Array<number | null>;
  kdjJ: Array<number | null>;
  rsi6: Array<number | null>;
};
