import type { DashboardViewState } from "../../stores/dashboardStore";

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
