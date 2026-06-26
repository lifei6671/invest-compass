import type { StockChartQuote } from "../types";

type QuoteSummaryStripProps = {
  quote: StockChartQuote;
};

const summaryItems: Array<{ key: keyof StockChartQuote; label: string; tone?: "up" | "down" }> = [
  { key: "open", label: "今日", tone: "down" },
  { key: "high", label: "最高", tone: "up" },
  { key: "low", label: "最低", tone: "down" },
  { key: "previousClose", label: "昨收" },
  { key: "volumeText", label: "成交量" },
  { key: "amountText", label: "成交额" },
  { key: "turnoverRateText", label: "换手率" },
] as const;

export function QuoteSummaryStrip(props: QuoteSummaryStripProps) {
  return (
    <section className="mx-4 mt-3 flex h-12 shrink-0 items-center rounded-lg border border-[#e5eaf3] bg-white px-4">
      <div className="flex items-center gap-11">
        {summaryItems.map((item) => (
          <div key={item.key} className="flex items-center gap-3">
            <span className="text-[13px] font-medium text-[#64748b]">{item.label}</span>
            <span className={["app-number text-[14px] font-semibold", valueClass(item.tone)].join(" ")}>
              {formatValue(props.quote[item.key])}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}

function formatValue(value: string | number) {
  return typeof value === "number" ? value.toFixed(2) : value;
}

function valueClass(tone?: "up" | "down") {
  if (tone === "up") {
    return "text-[#ff4d4f]";
  }
  if (tone === "down") {
    return "text-[#16a34a]";
  }
  return "text-[#111827]";
}
