import { Button, Tag } from "antd";
import { CopyOutlined, ExportOutlined } from "@ant-design/icons";
import type { NewsItem } from "../types";

type NewsListItemProps = {
  item: NewsItem;
  onOpenOriginal: (item: NewsItem) => void;
  onCopySummary: (item: NewsItem) => void;
};

const sourceInitialMap: Record<string, string> = {
  财联社: "C",
  财联社电报: "财",
  新浪财经: "新",
  "华尔街见闻-全球7x24": "华",
  "东方财富研报": "研",
  "东方财富行业研究": "行",
  东方财富公告: "公",
  证券时报: "证",
  芯榜: "芯",
  界面新闻: "界",
  同花顺资讯: "同",
  上海证券报: "S",
  "Wind 资讯": "W",
  第一财经: "一",
  证券日报: "日",
};

export function NewsListItem({ item, onOpenOriginal, onCopySummary }: NewsListItemProps) {
  return (
    <article className="news-list-item">
      <div className="news-source-cell">
        <span className="news-source-logo">{sourceInitialMap[item.source] ?? item.source.slice(0, 1)}</span>
        <div className="news-source-meta">
          <span className="news-source-name">{item.source}</span>
          <span className="news-source-time">{item.timeLabel}</span>
        </div>
      </div>
      <div className="news-item-content">
        <h3>{item.title}</h3>
        <p>{item.summary}</p>
        <div className="news-item-tags">
          {item.tags.map((tag) => (
            <Tag key={tag}>{tag}</Tag>
          ))}
        </div>
      </div>
      <div className="news-item-actions">
        <Button type="link" icon={<ExportOutlined />} onClick={() => onOpenOriginal(item)}>
          查看原文
        </Button>
        <Button type="text" icon={<CopyOutlined />} onClick={() => onCopySummary(item)}>
          复制摘要
        </Button>
      </div>
    </article>
  );
}
