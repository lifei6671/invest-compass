import { BarChartOutlined, FileTextOutlined, FundOutlined, LineChartOutlined, ProfileOutlined } from "@ant-design/icons";
import type { ContextSummary, DataModuleHeaderProps } from "../types";

type DataContextPreviewProps = {
  value: ContextSummary;
  onViewMoreNews: () => void;
};

export function DataContextPreview(props: DataContextPreviewProps) {
  const { value } = props;
  return (
    <section className="analysis-card analysis-context-preview">
      <h2 className="analysis-card-title">数据上下文预览</h2>
      <div className="analysis-context-module">
        <ModuleHeader icon={<ProfileOutlined />} title="基础信息摘要" tone="blue" />
        <div className="analysis-kv-list">
          <KV label="公司全称" value={value.basicInfo.companyName} />
          <KV label="所属行业" value={value.basicInfo.industry} />
          <KV label="上市日期" value={value.basicInfo.listDate} />
          <KV label="总市值" value={value.basicInfo.totalMarketCap} />
          <KV label="流通市值" value={value.basicInfo.floatMarketCap} />
        </div>
      </div>
      <div className="analysis-context-module">
        <ModuleHeader icon={<FundOutlined />} title="最新行情摘要" meta={value.quote.updateTime} tone="green" />
        <div className="analysis-metric-grid analysis-quote-grid">
          <Metric label="现价" value={value.quote.price} tone="up" />
          <Metric label="涨跌额" value={value.quote.changeAmount} tone="up" />
          <Metric label="涨跌幅" value={value.quote.changePercent} tone="up" />
          <Metric label="开盘" value={value.quote.open} tone="down" />
          <Metric label="最高" value={value.quote.high} tone="up" />
          <Metric label="最低" value={value.quote.low} tone="down" />
          <Metric label="成交额" value={value.quote.amount} />
          <Metric label="成交量" value={value.quote.volume} />
          <Metric label="换手率" value={value.quote.turnoverRate} />
        </div>
      </div>
      <div className="analysis-context-module">
        <ModuleHeader icon={<LineChartOutlined />} title="K线概况（日K）" tone="orange" />
        <div className="analysis-metric-grid">
          <Metric label="近20日涨跌幅" value={value.kline.change20d} tone="up" />
          <Metric label="60日涨跌幅" value={value.kline.change60d} tone="up" />
          <Metric label="年初至今" value={value.kline.ytdChange} tone="up" />
          <Metric label="20日均线" value={value.kline.ma20} />
          <Metric label="60日均线" value={value.kline.ma60} />
          <Metric label="120日均线" value={value.kline.ma120} />
        </div>
      </div>
      <div className="analysis-context-module">
        <ModuleHeader icon={<BarChartOutlined />} title="技术指标摘要" meta="最新" tone="purple" />
        <div className="analysis-indicator-list">
          <KV label="MA(5/10/20)" value={value.indicators.ma} />
          <KV label="MACD" value={value.indicators.macd} mixed />
          <KV label="RSI(6/12/24)" value={value.indicators.rsi} />
          <KV label="KDJ" value={value.indicators.kdj} mixed />
          <KV label="BOLL(20)" value={value.indicators.boll} mixed />
        </div>
      </div>
      <div className="analysis-context-module analysis-news-module">
        <ModuleHeader icon={<FileTextOutlined />} title="相关新闻摘要" tone="blue" />
        <div className="analysis-news-count">
          相关新闻数量 <strong>{value.news.count} 条</strong> <span>（{value.news.period}）</span>
        </div>
        <ul className="analysis-news-list">
          {value.news.items.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
        <button className="analysis-link-button analysis-news-link" type="button" onClick={props.onViewMoreNews}>
          查看更多新闻 &gt;
        </button>
      </div>
    </section>
  );
}

function ModuleHeader(props: DataModuleHeaderProps) {
  return (
    <header className="analysis-module-header">
      <span className={["analysis-module-icon", `analysis-module-icon-${props.tone ?? "blue"}`].join(" ")}>{props.icon}</span>
      <h3>{props.title}</h3>
      {props.meta ? <span className="analysis-module-meta">{props.meta}</span> : null}
    </header>
  );
}

function KV(props: { label: string; value: string; mixed?: boolean }) {
  return (
    <div className="analysis-kv-row">
      <span>{props.label}</span>
      <strong className={props.mixed ? "analysis-mixed-value" : undefined}>{props.value}</strong>
    </div>
  );
}

function Metric(props: { label: string; value: string; tone?: "up" | "down" }) {
  return (
    <div className="analysis-metric">
      <span>{props.label}</span>
      <strong className={props.tone ? `analysis-${props.tone}` : undefined}>{props.value}</strong>
    </div>
  );
}
