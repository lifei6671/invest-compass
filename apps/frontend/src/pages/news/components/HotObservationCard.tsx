import { FireFilled } from "@ant-design/icons";
import type { HotIndustry, MentionedStock, SentimentSummary } from "../types";

type HotObservationCardProps = {
  industries: HotIndustry[];
  mentionedStocks: MentionedStock[];
  sentiment: SentimentSummary;
};

export function HotObservationCard({ industries, mentionedStocks, sentiment }: HotObservationCardProps) {
  return (
    <section className="news-side-card">
      <header className="news-side-header">
        <div>
          <FireFilled className="news-hot-icon" />
          <h3>热点观察</h3>
        </div>
        <span>15:30 更新</span>
      </header>
      <div className="news-hot-section">
        <h4>今日热点行业 TOP5</h4>
        <div className="news-hot-industry-list">
          {industries.map((item) => (
            <div key={item.name} className="news-hot-industry-row">
              <span className={item.rank <= 2 ? "news-hot-rank news-hot-rank-red" : "news-hot-rank"}>{item.rank}</span>
              <span className="news-hot-name">{item.name}</span>
              <span className="news-hot-value">热度 {item.heat}</span>
              <span className="news-hot-bar">
                <i className={item.rank <= 2 ? "news-hot-bar-red" : undefined} style={{ width: `${item.heat}%` }} />
              </span>
            </div>
          ))}
        </div>
      </div>
      <div className="news-hot-section">
        <h4>高频提及股票（近 24h）</h4>
        <div className="news-mentioned-grid">
          {mentionedStocks.map((stock, index) => (
            <span key={stock.name} className="news-mentioned-tag">
              {stock.name}
              <strong className={index < 4 ? "news-mentioned-red" : undefined}>{stock.count}</strong>
            </span>
          ))}
        </div>
      </div>
      <div className="news-hot-section">
        <h4>市场情绪摘要</h4>
        <div className="news-sentiment-row">
          <span className="news-sentiment-positive">利好 {sentiment.positive.count}（{sentiment.positive.percent}%）</span>
          <span>中性 {sentiment.neutral.count}（{sentiment.neutral.percent}%）</span>
          <span className="news-sentiment-negative">利空 {sentiment.negative.count}（{sentiment.negative.percent}%）</span>
        </div>
        <p>{sentiment.summary}</p>
      </div>
    </section>
  );
}

