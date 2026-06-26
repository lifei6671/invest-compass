import { Empty, Pagination, Select } from "antd";
import type { NewsItem } from "../types";
import { NewsListItem } from "./NewsListItem";
import { PAGE_SIZE_OPTIONS } from "../../../lib/pagination";

type NewsListCardProps = {
  items: NewsItem[];
  totalCount: number;
  sortMode: string;
  currentPage: number;
  pageSize: number;
  onSortModeChange: (value: string) => void;
  onPageChange: (page: number, pageSize: number) => void;
  onOpenOriginal: (item: NewsItem) => void;
  onCopySummary: (item: NewsItem) => void;
  emptyDescription?: string;
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
            options={[{ value: "按最新", label: "按最新" }]}
            onChange={props.onSortModeChange}
          />
        </div>
      </header>
      <div className="news-list-body">
        {props.items.length > 0 ? (
          props.items.map((item) => (
            <NewsListItem
              key={item.id}
              item={item}
              onOpenOriginal={props.onOpenOriginal}
              onCopySummary={props.onCopySummary}
            />
          ))
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={props.emptyDescription ?? "暂无匹配资讯"} />
        )}
      </div>
      <footer className="news-pagination-row">
        <span>共 {props.totalCount} 条</span>
        <Pagination
          current={props.currentPage}
          total={props.totalCount}
          pageSize={props.pageSize}
          size="small"
          showSizeChanger
          pageSizeOptions={[...PAGE_SIZE_OPTIONS]}
          showQuickJumper={{ goButton: "页" }}
          onChange={props.onPageChange}
        />
      </footer>
    </section>
  );
}
