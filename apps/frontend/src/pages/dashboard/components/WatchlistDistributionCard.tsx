import { InfoCircleOutlined } from "@ant-design/icons";
import type { WatchlistDistribution } from "../types";

const emptyDistribution: WatchlistDistribution = {
  total: 0,
  up: { count: 0, percent: 0 },
  down: { count: 0, percent: 0 },
  flat: { count: 0, percent: 0 },
  avgChangePercent: "--",
  upProbability: "--",
  changeFromYesterday: "--",
};

export function WatchlistDistributionCard() {
  const data = emptyDistribution;

  return (
    <section className="dashboard-surface dashboard-distribution-card">
      <div className="dashboard-card-title">
        <h2>自选股涨跌分布</h2>
        <InfoCircleOutlined />
      </div>
      <div className="dashboard-distribution-body">
        <DonutChart />
        <div className="dashboard-distribution-legend">
          <LegendRow color="#ff4d4f" label="上涨" count={data.up.count} percent={data.up.percent} />
          <LegendRow color="#16a34a" label="下跌" count={data.down.count} percent={data.down.percent} />
          <LegendRow color="#c4c9d4" label="平盘" count={data.flat.count} percent={data.flat.percent} />
        </div>
        <div className="dashboard-market-summary">
          <div className="dashboard-market-summary-title">
            今日总体表现
            <InfoCircleOutlined />
          </div>
          <Metric label="平均涨跌幅" value={data.avgChangePercent} />
          <Metric label="上涨概率" value={data.upProbability} />
          <Metric label="较昨日变化" value={data.changeFromYesterday} />
        </div>
      </div>
    </section>
  );
}

function DonutChart() {
  const { up, down, flat, total } = emptyDistribution;
  const segments = [
    { value: up.percent, color: "#ff4d4f" },
    { value: down.percent, color: "#16a34a" },
    { value: flat.percent, color: "#c4c9d4" },
  ];
  let offset = 25;

  return (
    <div className="dashboard-donut-wrap">
      <svg className="dashboard-donut" viewBox="0 0 140 140" aria-hidden="true">
        <circle cx="70" cy="70" fill="none" r="52" stroke="#edf1f7" strokeWidth="18" />
        {segments.map((segment) => {
          const dashOffset = offset;
          offset -= segment.value;
          return (
            <circle
              key={segment.color}
              cx="70"
              cy="70"
              fill="none"
              pathLength="100"
              r="52"
              stroke={segment.color}
              strokeDasharray={`${segment.value} ${100 - segment.value}`}
              strokeDashoffset={dashOffset}
              strokeLinecap="butt"
              strokeWidth="18"
              transform="rotate(-90 70 70)"
            />
          );
        })}
      </svg>
      <div className="dashboard-donut-center">
        <span>总数</span>
        <strong>{total}</strong>
      </div>
    </div>
  );
}

function LegendRow(props: { color: string; label: string; count: number; percent: number }) {
  return (
    <div className="dashboard-legend-row">
      <span className="dashboard-legend-dot" style={{ backgroundColor: props.color }} />
      <span className="dashboard-legend-label">{props.label}</span>
      <strong>{props.count}</strong>
      <span>{props.percent.toFixed(2)}%</span>
    </div>
  );
}

function Metric(props: { label: string; value: string }) {
  return (
    <div className="dashboard-metric">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
