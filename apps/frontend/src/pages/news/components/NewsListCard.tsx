import { Button, Empty, Pagination, Select } from "antd";
import { UnorderedListOutlined } from "@ant-design/icons";
import type { NewsItem } from "../types";
import { NewsListItem } from "./NewsListItem";

type NewsListCardProps = {
  items: NewsItem[];
  totalCount: number;
  sortMode: string;
  currentPage: number;
  onSortModeChange: (value: string) => void;
  onViewSwitch: () => void;
  onPageChange: (page: number) => void;
  onOpenOriginal: (item: NewsItem) => void;
  onAddContext: (item: NewsItem) => void;
  onCopySummary: (item: NewsItem) => void;
};

export function NewsListCard(props: NewsListCardProps) {
  return (
    <section className="news-list-card">
      <header className="news-list-header">
        <div className="news-list-title">
          <h2>资讯列表</h2>
          <span>（共 {props.totalCount} 条）</span>
        </div>
        <div className="news-list-tools">
          <Select
            value={props.sortMode}
            className="news-sort-select"
            options={["按最新", "按热度", "按相关性"].map((value) => ({ value, label: value }))}
            onChange={props.onSortModeChange}
          />
          <Button aria-label="切换资讯视图" icon={<UnorderedListOutlined />} onClick={props.onViewSwitch} />
        </div>
      </header>
      <div className="news-list-body">
        {props.items.length > 0 ? (
          props.items.map((item) => (
            <NewsListItem
              key={item.id}
              item={item}
              onOpenOriginal={props.onOpenOriginal}
              onAddContext={props.onAddContext}
              onCopySummary={props.onCopySummary}
            />
          ))
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无匹配资讯" />
        )}
      </div>
      <footer className="news-pagination-row">
        <span>共 {props.totalCount} 条</span>
        <Pagination current={props.currentPage} total={218} pageSize={10} size="small" showSizeChanger={false} onChange={props.onPageChange} />
        <span>10 条/页</span>
        <span>跳至 1 页</span>
      </footer>
    </section>
  );
}

