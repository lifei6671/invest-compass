import { Button, ConfigProvider, Empty, Input, Pagination, Select } from "antd";
import { FilterOutlined, ReloadOutlined, SearchOutlined } from "@ant-design/icons";
import { appAntdLocale } from "../../lib/antdLocale";
import { PAGE_SIZE_OPTIONS } from "../../lib/pagination";
import { WatchlistStockCard } from "./WatchlistStockCard";
import type { WatchlistItem } from "./types";

type FilterOption = {
  value: string;
  label: string;
};

type WatchlistCardGridProps = {
  items: WatchlistItem[];
  keyword: string;
  onKeywordChange: (value: string) => void;
  onAdd: () => void;
  onDelete: (item: WatchlistItem) => void;
  onEdit: (item: WatchlistItem) => void;
  onView: (item: WatchlistItem) => void;
  onAnalyze: (item: WatchlistItem) => void;
  onRefresh: () => void;
  onSearch: () => void;
  isSearching?: boolean;
  emptyDescription?: string;
  totalCount: number;
  currentPage: number;
  pageSize: number;
  onPageChange: (page: number, pageSize: number) => void;
  marketFilter: string;
  tagFilter: string;
  marketOptions: FilterOption[];
  tagOptions: FilterOption[];
  onMarketFilterChange: (value: string) => void;
  onTagFilterChange: (value: string) => void;
};

export function WatchlistCardGrid(props: WatchlistCardGridProps) {
  return (
    <section className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <div className="mb-[18px] flex min-w-0 shrink-0 items-center gap-3 overflow-hidden rounded-xl border border-[#e5eaf3] bg-white px-4 py-4 shadow-[0_4px_18px_rgba(15,23,42,0.04)]">
        <Input
          allowClear
          className="h-9 w-[230px] shrink-0 rounded-md"
          style={{ flex: "0 0 230px", width: 230 }}
          placeholder="搜索自选股"
          prefix={<SearchOutlined className="text-slate-400" />}
          value={props.keyword}
          onChange={(event) => props.onKeywordChange(event.target.value)}
          onPressEnter={props.onSearch}
        />
        <Button type="primary" className="h-9 shrink-0 px-4" onClick={props.onAdd}>
          + 添加自选
        </Button>
        <Button className="h-9 shrink-0 px-4" icon={<ReloadOutlined />} loading={props.isSearching} onClick={props.onRefresh}>
          批量刷新
        </Button>
        <div className="ml-auto flex shrink-0 items-center gap-3">
          <Select className="w-[124px]" value={props.marketFilter} options={props.marketOptions} onChange={props.onMarketFilterChange} />
          <Select className="w-[124px]" value="default" options={[{ value: "default", label: "默认排序" }]} />
          <Select className="w-[124px]" value={props.tagFilter} suffixIcon={<FilterOutlined />} options={props.tagOptions} onChange={props.onTagFilterChange} />
        </div>
      </div>

      {props.items.length > 0 ? (
        <div className="grid min-w-0 grid-cols-[repeat(auto-fill,280px)] justify-start gap-4 overflow-x-auto pt-1 pb-1">
          {props.items.map((item) => (
            <WatchlistStockCard key={item.id} item={item} onDelete={props.onDelete} onEdit={props.onEdit} onView={props.onView} onAnalyze={props.onAnalyze} />
          ))}
        </div>
      ) : (
        <div className="flex min-h-[280px] items-center justify-center rounded-xl border border-dashed border-[#dbe5f2] bg-white">
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={props.emptyDescription ?? "暂无匹配自选股"} />
        </div>
      )}

      <div className="mt-4 flex shrink-0 items-center justify-between">
        <span className="text-[14px] text-slate-700">共 {props.totalCount} 条</span>
        <ConfigProvider locale={appAntdLocale}>
          <Pagination
            className="watchlist-pagination"
            current={props.currentPage}
            total={props.totalCount}
            pageSize={props.pageSize}
            showQuickJumper
            showSizeChanger
            pageSizeOptions={[...PAGE_SIZE_OPTIONS]}
            onChange={props.onPageChange}
          />
        </ConfigProvider>
      </div>
    </section>
  );
}
