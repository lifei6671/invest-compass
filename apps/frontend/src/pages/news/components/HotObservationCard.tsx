import { FireFilled } from "@ant-design/icons";
import type { HotIndustry, MentionedStock, SentimentSummary } from "../types";

type HotObservationCardProps = {
  industries: HotIndustry[];
  mentionedStocks: MentionedStock[];
  sentiment: SentimentSummary;
  updatedAt?: string;
};

export function HotObservationCard({ industries, mentionedStocks, sentiment, updatedAt }: HotObservationCardProps) {
  return (
    <section className="news-side-card">
      <header className="news-side-header">
        <div>
          <FireFilled className="news-hot-icon" />
          <h3>热点观察</h3>
        </div>
        <span>{updatedAt ? `${updatedAt} 更新` : "暂无更新时间"}</span>
      </header>
      <div className="news-hot-section">
        <h4>缓存热点标签 TOP5</h4>
        <div className="news-hot-industry-list">
          {industries.length > 0 ? (
            industries.map((item) => (
              <div key={item.name} className="news-hot-industry-row">
                <span className={item.rank <= 2 ? "news-hot-rank news-hot-rank-red" : "news-hot-rank"}>{item.rank}</span>
                <span className="news-hot-name">{item.name}</span>
                <span className="news-hot-value">热度 {item.heat}</span>
                <span className="news-hot-bar">
                  <i className={item.rank <= 2 ? "news-hot-bar-red" : undefined} style={{ width: `${item.heat}%` }} />
                </span>
              </div>
            ))
          ) : (
            <p className="news-side-empty">暂无缓存热点标签</p>
          )}
        </div>
      </div>
      <div className="news-hot-section">
        <h4>缓存高频提及股票</h4>
        <div className="news-mentioned-grid">
          {mentionedStocks.length > 0 ? (
            mentionedStocks.map((stock, index) => (
              <span key={stock.name} className="news-mentioned-tag">
                {stock.name}
                <strong className={index < 4 ? "news-mentioned-red" : undefined}>{stock.count}</strong>
              </span>
            ))
          ) : (
            <p className="news-side-empty">暂无缓存提及股票</p>
          )}
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
