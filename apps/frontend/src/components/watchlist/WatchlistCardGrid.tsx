import { App as AntApp, Button, ConfigProvider, Input, Pagination, Select } from "antd";
import { FilterOutlined, ReloadOutlined, SearchOutlined } from "@ant-design/icons";
import { appAntdLocale } from "../../lib/antdLocale";
import { WatchlistStockCard } from "./WatchlistStockCard";
import type { WatchlistItem } from "./mock";

type WatchlistCardGridProps = {
  items: WatchlistItem[];
  keyword: string;
  onKeywordChange: (value: string) => void;
  onAdd: () => void;
  onDelete: (item: WatchlistItem) => void;
  onView: (item: WatchlistItem) => void;
  onRefresh: () => void;
};

const cardOrder = [1, 2, 3, 8, 5, 6, 7, 10];

export function WatchlistCardGrid(props: WatchlistCardGridProps) {
  const { message } = AntApp.useApp();
  const orderedItems = [...props.items].sort((left, right) => {
    const leftIndex = cardOrder.indexOf(left.id);
    const rightIndex = cardOrder.indexOf(right.id);
    const normalizedLeft = leftIndex === -1 ? Number.MAX_SAFE_INTEGER : leftIndex;
    const normalizedRight = rightIndex === -1 ? Number.MAX_SAFE_INTEGER : rightIndex;
    return normalizedLeft - normalizedRight || left.id - right.id;
  });
  const visibleItems = orderedItems.slice(0, 8);

  const handleFilterChange = () => {
    message.info("筛选功能待接入");
  };

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
        />
        <Button type="primary" className="h-9 shrink-0 px-4" onClick={props.onAdd}>
          + 添加自选
        </Button>
        <Button className="h-9 shrink-0 px-4" icon={<ReloadOutlined />} onClick={props.onRefresh}>
          批量刷新
        </Button>
        <div className="ml-auto flex shrink-0 items-center gap-3">
          <Select className="w-[124px]" value="all" options={[{ value: "all", label: "全部市场" }]} onChange={handleFilterChange} />
          <Select className="w-[124px]" value="default" options={[{ value: "default", label: "默认排序" }]} onChange={handleFilterChange} />
          <Select className="w-[124px]" value="tag" suffixIcon={<FilterOutlined />} options={[{ value: "tag", label: "标签筛选" }]} onChange={handleFilterChange} />
        </div>
      </div>

      <div className="grid min-w-0 grid-cols-[repeat(auto-fill,256px)] justify-start gap-4 overflow-x-auto pt-1 pb-1">
        {visibleItems.map((item) => (
          <WatchlistStockCard key={item.id} item={item} onDelete={props.onDelete} onView={props.onView} />
        ))}
      </div>

      <div className="mt-4 flex shrink-0 items-center justify-between">
        <span className="text-[14px] text-slate-700">共 56 条</span>
        <ConfigProvider locale={appAntdLocale}>
          <Pagination className="watchlist-pagination" current={1} total={56} pageSize={10} showQuickJumper showSizeChanger pageSizeOptions={[10]} onChange={() => undefined} />
        </ConfigProvider>
      </div>
    </section>
  );
}
