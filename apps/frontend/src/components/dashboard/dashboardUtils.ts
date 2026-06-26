import type { DashboardViewState } from "../../stores/dashboardStore";

export type ChinaMarketSessionStatus = "waiting" | "trading" | "closed";

export function latestDashboardQuoteTime(state: DashboardViewState | null) {
  if (!state) {
    return null;
  }
  return [...state.indexQuotes.map((item) => item.quote?.quote_time), ...state.watchlistRows.map((item) => item.quote?.quote_time)]
    .filter((value): value is string => Boolean(value))
    .sort()
    .at(-1) ?? null;
}

export function formatClock(value: string | null | undefined) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleTimeString("zh-CN", { hour12: false });
}

export function chinaMarketSessionStatus(quoteTime: string | null | undefined, now = new Date()): ChinaMarketSessionStatus {
  const quoteParts = quoteTime ? chinaTimeParts(new Date(quoteTime)) : null;
  if (!quoteParts) {
    return "waiting";
  }
  const nowParts = chinaTimeParts(now);
  if (!nowParts || quoteParts.date !== nowParts.date || !isChinaTradingWeekday(nowParts.weekday)) {
    return "closed";
  }
  const minutes = nowParts.hour * 60 + nowParts.minute;
  if ((minutes >= 9 * 60 + 30 && minutes < 11 * 60 + 30) || (minutes >= 13 * 60 && minutes < 15 * 60)) {
    return "trading";
  }
  if (minutes < 9 * 60 + 30) {
    return "waiting";
  }
  return "closed";
}

export function chinaMarketSessionLabel(status: ChinaMarketSessionStatus) {
  switch (status) {
    case "trading":
      return "交易中";
    case "waiting":
      return "未开盘";
    case "closed":
      return "已收盘";
  }
}

type ChinaTimeParts = {
  date: string;
  weekday: string;
  hour: number;
  minute: number;
};

const chinaDateTimeFormatter = new Intl.DateTimeFormat("en-CA", {
  timeZone: "Asia/Shanghai",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  weekday: "short",
  hour12: false,
});

function chinaTimeParts(date: Date): ChinaTimeParts | null {
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  const parts = Object.fromEntries(chinaDateTimeFormatter.formatToParts(date).map((part) => [part.type, part.value]));
  const hour = Number(parts.hour === "24" ? "0" : parts.hour);
  const minute = Number(parts.minute);
  if (!parts.year || !parts.month || !parts.day || !parts.weekday || Number.isNaN(hour) || Number.isNaN(minute)) {
    return null;
  }
  return {
    date: `${parts.year}-${parts.month}-${parts.day}`,
    weekday: parts.weekday,
    hour,
    minute,
  };
}

function isChinaTradingWeekday(weekday: string) {
  return weekday !== "Sat" && weekday !== "Sun";
}
