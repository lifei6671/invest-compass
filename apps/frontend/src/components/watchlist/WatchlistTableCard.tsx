import { Button, ConfigProvider, Input, Pagination, Select, Table, Tag, Tooltip, type TableColumnsType } from "antd";
import { DeleteOutlined, EditOutlined, EyeOutlined, FilterOutlined, ReloadOutlined, RobotOutlined, SearchOutlined, SettingOutlined, StarOutlined } from "@ant-design/icons";
import { appAntdLocale } from "../../lib/antdLocale";
import { PAGE_SIZE_OPTIONS } from "../../lib/pagination";
import type { WatchlistItem } from "./types";

type FilterOption = {
  value: string;
  label: string;
};

type WatchlistTableCardProps = {
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

const tagClassByName: Record<string, string> = {
  核心: "border-0 bg-[#e9f8ef] text-[#18a058]",
  光模块: "border-0 bg-[#eaf2ff] text-[#1677ff]",
  PCB: "border-0 bg-[#fff2e6] text-[#f97316]",
  半导体: "border-0 bg-[#f0ecff] text-[#635bff]",
  芯片: "border-0 bg-[#fff0f0] text-[#ff4d4f]",
  设备: "border-0 bg-[#f2edff] text-[#7c3aed]",
  新能源: "border-0 bg-[#eaf2ff] text-[#1677ff]",
  白马: "border-0 bg-[#eff6ff] text-[#2563eb]",
  元器件: "border-0 bg-[#f2edff] text-[#7c3aed]",
};

export function WatchlistTableCard(props: WatchlistTableCardProps) {
  const columns: TableColumnsType<WatchlistItem> = [
    {
      title: "",
      dataIndex: "starred",
      width: 36,
      fixed: "left",
      render: (value: boolean) => <StarOutlined className={value ? "text-[#f59e0b]" : "text-slate-300"} />,
    },
    { title: "股票名称", dataIndex: "name", width: 86, fixed: "left", render: (value: string) => <span className="font-medium text-slate-900">{value}</span> },
    { title: "代码", dataIndex: "code", width: 84, render: (value: string) => <span className="app-number text-slate-700">{value}</span> },
    { title: "市场", dataIndex: "market", width: 54 },
    { title: "现价", dataIndex: "price", width: 76, render: (value: string) => <span className="app-number text-slate-900">{value}</span> },
    { title: "涨跌额", dataIndex: "changeAmount", width: 76, render: (value: string) => <span className={["app-number", valueTone(value)].join(" ")}>{value}</span> },
    { title: "涨跌幅", dataIndex: "changePercent", width: 76, render: (value: string) => <span className={["app-number", valueTone(value)].join(" ")}>{value}</span> },
    { title: "成交额", dataIndex: "amount", width: 76, render: (value: string) => <span className="app-number">{value}</span> },
    { title: "换手率", dataIndex: "turnoverRate", width: 68, render: (value: string) => <span className="app-number">{value}</span> },
    { title: "市盈率(PE)", dataIndex: "pe", width: 84, render: (value: string) => <span className="app-number">{value}</span> },
    { title: "所属行业", dataIndex: "industry", width: 78, ellipsis: true },
    {
      title: "标签",
      dataIndex: "tags",
      width: 92,
      render: (tags: string[]) => (
        <div className="flex flex-wrap gap-1">
          {tags.map((tag) => (
            <Tag key={tag} className={["m-0 rounded-md px-2 py-0.5 text-[13px]", tagClassByName[tag] ?? "border-0 bg-slate-100 text-slate-600"].join(" ")}>
              {tag}
            </Tag>
          ))}
        </div>
      ),
    },
    { title: "我的备注", dataIndex: "note", width: 92, ellipsis: true },
    { title: "最后更新时间", dataIndex: "updatedAt", width: 96, render: (value: string) => <span className="app-number text-slate-600">{value}</span> },
    {
      title: (
        <span className="flex items-center justify-center">
          <SettingOutlined />
        </span>
      ),
      key: "actions",
      width: 112,
      fixed: "right",
      render: (_, record) => (
        <div className="flex items-center justify-center gap-1">
          <Tooltip title="查看">
            <Button aria-label={`查看 ${record.name}`} type="text" size="small" icon={<EyeOutlined />} onClick={() => props.onView(record)} />
          </Tooltip>
          <Tooltip title="AI 分析">
            <Button aria-label={`AI 分析 ${record.name}`} type="text" size="small" className="text-[#1677ff]" icon={<RobotOutlined />} onClick={() => props.onAnalyze(record)} />
          </Tooltip>
          <Tooltip title="编辑">
            <Button aria-label={`编辑 ${record.name}`} type="text" size="small" icon={<EditOutlined />} onClick={() => props.onEdit(record)} />
          </Tooltip>
          <Tooltip title="删除">
            <Button aria-label={`删除 ${record.name}`} type="text" size="small" danger icon={<DeleteOutlined />} onClick={() => props.onDelete(record)} />
          </Tooltip>
        </div>
      ),
    },
  ];

  return (
    <section className="watchlist-table-card flex min-h-0 min-w-0 flex-1 flex-col rounded-xl border border-[#e5eaf3] bg-white shadow-[0_4px_18px_rgba(15,23,42,0.04)]">
      <div className="flex shrink-0 items-center gap-3 border-b border-[#edf1f7] px-4 py-4">
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
      <Table
        rowKey="id"
        className="watchlist-table min-h-0 flex-1"
        columns={columns}
        dataSource={props.items}
        pagination={false}
        tableLayout="fixed"
        scroll={{ x: 1184, y: "calc(100vh - 470px)" }}
        locale={{ emptyText: props.emptyDescription ?? "暂无匹配自选股" }}
      />
      <div className="flex shrink-0 items-center justify-between px-4 py-4">
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

function valueTone(value: string) {
  if (value.startsWith("+")) {
    return "text-[#ff4d4f]";
  }
  if (value.startsWith("-")) {
    return "text-[#16a34a]";
  }
  return "text-slate-700";
}
