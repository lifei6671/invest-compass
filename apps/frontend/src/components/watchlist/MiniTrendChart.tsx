import { useId } from "react";

export type MiniTrendChartProps = {
  trend: "up" | "down";
  points?: number[];
};

const defaultPoints = {
  up: [20, 22, 21, 25, 24, 27, 23, 24, 22, 26, 28, 30],
  down: [30, 28, 27, 25, 26, 22, 20, 19, 21, 18, 20, 23],
};

export function MiniTrendChart(props: MiniTrendChartProps) {
  const id = useId();
  const points = props.points?.length ? props.points : defaultPoints[props.trend];
  const min = Math.min(...points);
  const max = Math.max(...points);
  const range = Math.max(max - min, 1);
  const linePoints = points
    .map((point, index) => {
      const x = (index / Math.max(points.length - 1, 1)) * 96 + 2;
      const y = 36 - ((point - min) / range) * 28;
      return `${x.toFixed(2)},${y.toFixed(2)}`;
    })
    .join(" ");
  const first = linePoints.split(" ")[0];
  const last = linePoints.split(" ").at(-1);
  const color = props.trend === "up" ? "#ff4d4f" : "#16a34a";
  const gradientId = `watchlist-trend-${props.trend}-${id.replace(/:/g, "")}`;

  return (
    <svg aria-label={props.trend === "up" ? "上涨走势" : "下跌走势"} className="h-12 w-24" viewBox="0 0 100 40" role="img">
      <defs>
        <linearGradient id={gradientId} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.16" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      {first && last ? <polygon fill={`url(#${gradientId})`} points={`${first} ${linePoints} ${last.split(",")[0]},40 ${first.split(",")[0]},40`} /> : null}
      <polyline fill="none" points={linePoints} stroke={color} strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
    </svg>
  );
}
