import { useId } from "react";
import type { MarketIndexTrend } from "../types";

type MiniSparklineProps = {
  values: number[];
  trend: MarketIndexTrend;
};

export function MiniSparkline({ values, trend }: MiniSparklineProps) {
  const gradientId = `sparkline-${useId().replace(/:/g, "")}`;
  const color = trend === "up" ? "#ff4d4f" : "#16a34a";
  const width = 120;
  const height = 52;
  const padding = 4;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = Math.max(1, max - min);
  const points = values
    .map((value, index) => {
      const x = (index / Math.max(1, values.length - 1)) * (width - padding * 2) + padding;
      const y = height - padding - ((value - min) / range) * (height - padding * 2);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
  const fillPoints = `${padding},${height - padding} ${points} ${width - padding},${height - padding}`;

  return (
    <svg className="dashboard-sparkline" viewBox={`0 0 ${width} ${height}`} aria-hidden="true">
      <defs>
        <linearGradient id={gradientId} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.16" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      <polygon fill={`url(#${gradientId})`} points={fillPoints} />
      <polyline fill="none" points={points} stroke={color} strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
    </svg>
  );
}
