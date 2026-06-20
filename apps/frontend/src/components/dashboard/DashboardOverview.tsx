import { Badge, Button, Card, Spin, Tabs, Tag, Typography } from "antd";
import { FileDoneOutlined, FileTextOutlined, LineChartOutlined, StarOutlined } from "@ant-design/icons";
import { useEffect, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { useDashboardStore, type DashboardQuoteState, type DashboardViewState, type DashboardWatchlistRow } from "../../stores/dashboardStore";
import type { MarketQuote } from "../../services/coreClient";

export function DashboardOverview() {
  const state = useDashboardStore((store) => store.state);
  const loading = useDashboardStore((store) => store.loading);
  const error = useDashboardStore((store) => store.error);
  const load = useDashboardStore((store) => store.load);

  useEffect(() => {
    void load();
  }, [load]);

  if (error) {
    return <div className="rounded-lg border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-600">总览读取失败：{error}</div>;
  }
  if (!state) {
    return (
      <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-sm text-blue-700">
        正在连接本地核心服务 {loading ? <Spin className="ml-2" size="small" /> : null}
      </div>
    );
  }

  return (
    <section className="flex min-h-[calc(100vh-148px)] flex-col">
      <div className="mb-6">
        <Typography.Title level={1} className="!mb-2 !text-[28px] !leading-9 !text-slate-950">
          欢迎使用投研罗盘
        </Typography.Title>
        <Typography.Text className="text-[18px] text-slate-500">本地优先的 AI 投研桌面工作台</Typography.Text>
      </div>

      <div className="grid grid-cols-1 gap-3 xl:grid-cols-2">
        <MarketOverviewCard state={state} />
        <WatchlistSnapshotCard rows={state.watchlistRows} />
        <RecentReportsCard state={state} />
        <RecentTasksCard state={state} />
      </div>

      <div className="mt-auto pt-8 text-right text-[15px] font-medium text-slate-400">仅供研究，不构成投资建议</div>
    </section>
  );
}

export function latestDashboardQuoteTime(state: DashboardViewState | null) {
  if (!state) {
    return null;
  }
  return [...state.indexQuotes.map((item) => item.quote?.quote_time), ...state.watchlistRows.map((item) => item.quote?.quote_time)]
    .filter((value): value is string => Boolean(value))
    .sort()
    .at(-1) ?? null;
}

export function formatClock(value: string | null | undefined) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleTimeString("zh-CN", { hour12: false });
}

function MarketOverviewCard(props: { state: DashboardViewState }) {
  const statisticRows = [
    { label: "上涨家数", value: props.state.summary.watchlist.up_count, tone: "up" },
    { label: "下跌家数", value: props.state.summary.watchlist.down_count, tone: "down" },
    { label: "平盘家数", value: props.state.summary.watchlist.flat_count, tone: "flat" },
    { label: "成交额", value: marketAmountText(props.state.indexQuotes), tone: "flat" },
    { label: "北向净流入", value: "暂未接入", tone: "flat" },
  ];
  return (
    <DashboardCard icon={<LineChartOutlined />} title="市场概览" action={<CardAction label="更多" disabledReason="市场详情接口暂未接入" />} className="min-h-[356px]">
      <Tabs
        className="dashboard-tabs"
        defaultActiveKey="indexes"
        items={[
          {
            key: "indexes",
            label: "主要指数",
            children: (
              <div className="grid grid-cols-4 gap-2">
                {props.state.indexQuotes.map((item) => (
                  <IndexQuoteTile key={item.symbol} item={item} />
                ))}
              </div>
            ),
          },
          { key: "sectors", label: "行业板块", children: <EmptyDashboardPanel text="暂无行业板块数据" /> },
          { key: "distribution", label: "涨跌分布", children: <EmptyDashboardPanel text="暂无涨跌分布数据" /> },
          { key: "sentiment", label: "市场情绪", children: <EmptyDashboardPanel text="暂无市场情绪数据" /> },
        ]}
      />
      <div className="mt-4 grid grid-cols-5 rounded-md border border-[#e3e9f2] bg-white">
        {statisticRows.map((item) => (
          <div key={item.label} className="flex min-h-[60px] flex-col items-center justify-center border-r border-[#e3e9f2] last:border-r-0">
            <div className="text-[14px] text-slate-500">{item.label}</div>
            <div className={["mt-1 text-[17px] font-semibold", valueToneClass(item.tone)].join(" ")}>{item.value}</div>
          </div>
        ))}
      </div>
      <div className="mt-4 text-[13px] text-slate-400">数据更新时间：{formatDateTime(latestDashboardQuoteTime(props.state)) || "暂无"}</div>
    </DashboardCard>
  );
}

function WatchlistSnapshotCard(props: { rows: DashboardWatchlistRow[] }) {
  return (
    <DashboardCard icon={<StarOutlined />} title="我的自选" action={<CardAction label="管理" to="/watchlist" />} className="min-h-[356px]">
      <DashboardGridHeader columns="grid-cols-[1.4fr_1fr_1fr_1fr]" labels={["名称", "代码", "最新", "涨跌幅"]} />
      {props.rows.length > 0 ? (
        <div className="overflow-hidden rounded-b-md border-x border-b border-[#e3e9f2]">
          {props.rows.map((row) => (
            <div key={row.id} className="grid min-h-[42px] grid-cols-[1.4fr_1fr_1fr_1fr] items-center border-t border-[#e3e9f2] px-3 text-[14px] first:border-t-0">
              <div className="flex items-center gap-3 font-medium text-slate-700">
                <StarOutlined className="text-slate-300" />
                <span className="truncate">{row.symbol}</span>
              </div>
              <div className="text-slate-700">{row.symbol}</div>
              <div className="font-medium text-slate-800">{row.quote ? formatNumber(row.quote.price) : "--"}</div>
              <div className={valueToneClass(percentTone(row.quote?.change_percent))}>{formatPercent(row.quote?.change_percent)}</div>
            </div>
          ))}
        </div>
      ) : (
        <EmptyDashboardPanel text="暂无自选股" />
      )}
      <div className="mt-4 flex items-center justify-between text-[13px] text-slate-400">
        <span>更新于 {formatClock(latestWatchlistQuoteTime(props.rows)) || "--:--:--"}</span>
        <CardAction label="查看全部" to="/watchlist" />
      </div>
    </DashboardCard>
  );
}

function RecentReportsCard(props: { state: DashboardViewState }) {
  return (
    <DashboardCard icon={<FileTextOutlined />} title="最近报告" action={<CardAction label="更多" to="/reports" />} className="min-h-[336px]">
      <DashboardGridHeader columns="grid-cols-[2fr_0.9fr_1fr_0.8fr]" labels={["报告标题", "标签", "更新时间", "状态"]} />
      {props.state.summary.recent_reports.length > 0 ? (
        <div className="overflow-hidden rounded-b-md border-x border-b border-[#e3e9f2]">
          {props.state.summary.recent_reports.map((report) => (
            <div key={report.id} className="grid min-h-[42px] grid-cols-[2fr_0.9fr_1fr_0.8fr] items-center border-t border-[#e3e9f2] px-3 text-[14px] first:border-t-0">
              <div className="truncate font-medium text-slate-700">{report.title}</div>
              <div>
                <Tag className="m-0 rounded-md border-0 bg-[#eaf2ff] text-[#1677ff]">{report.analysis_type || "公司研究"}</Tag>
              </div>
              <div className="text-slate-600">{formatShortDateTime(report.updated_at || report.created_at)}</div>
              <div className="flex items-center gap-2 text-slate-700">
                <Badge status="success" />
                已完成
              </div>
            </div>
          ))}
        </div>
      ) : (
        <EmptyDashboardPanel text="暂无分析报告" />
      )}
      <DashboardRiskTips tips={props.state.summary.risk_tips} />
    </DashboardCard>
  );
}

function RecentTasksCard(props: { state: DashboardViewState }) {
  return (
    <DashboardCard icon={<FileDoneOutlined />} title="最近任务" action={<CardAction label="更多" to="/tasks" />} className="min-h-[336px]">
      <DashboardGridHeader columns="grid-cols-[2fr_1fr_0.9fr_1fr]" labels={["任务名称", "类型", "状态", "时间"]} />
      {props.state.summary.recent_tasks.length > 0 ? (
        <div className="overflow-hidden rounded-b-md border-x border-b border-[#e3e9f2]">
          {props.state.summary.recent_tasks.map((task) => (
            <div key={task.id} className="grid min-h-[42px] grid-cols-[2fr_1fr_0.9fr_1fr] items-center border-t border-[#e3e9f2] px-3 text-[14px] first:border-t-0">
              <div className="truncate font-medium text-slate-700">{task.title}</div>
              <div>
                <Tag className="m-0 rounded-md border-0 bg-[#eef2ff] text-[#4f46e5]">{task.type || "AI分析"}</Tag>
              </div>
              <div className="flex items-center gap-2 text-slate-700">
                <Badge status={task.status === "RUNNING" ? "processing" : task.status === "SUCCESS" ? "success" : "error"} />
                {taskStatusText(task.status)}
              </div>
              <div className="text-slate-600">{formatShortDateTime(task.updated_at || task.created_at)}</div>
            </div>
          ))}
        </div>
      ) : (
        <EmptyDashboardPanel text="暂无任务记录" />
      )}
    </DashboardCard>
  );
}

function DashboardCard(props: { title: string; icon: ReactNode; action?: ReactNode; className?: string; children: ReactNode }) {
  return (
    <Card className={["dashboard-card rounded-lg border-[#dce5ef] shadow-[0_3px_12px_rgba(15,23,42,0.06)]", props.className ?? ""].join(" ")}>
      <div className="mb-4 flex items-center justify-between border-b border-[#e3e9f2] pb-4">
        <div className="flex items-center gap-3 text-[18px] font-semibold text-slate-950">
          <span className="text-[21px] text-slate-600">{props.icon}</span>
          {props.title}
        </div>
        {props.action}
      </div>
      {props.children}
    </Card>
  );
}

function CardAction(props: { label: string; to?: string; disabledReason?: string }) {
  const button = (
    <Button type="link" disabled={Boolean(props.disabledReason)} className="h-auto px-0 text-[14px] font-medium text-[#1677ff]">
      {props.label}
      <span className="ml-1">›</span>
    </Button>
  );
  if (props.to) {
    return <Link to={props.to}>{button}</Link>;
  }
  return (
    <span title={props.disabledReason}>{button}</span>
  );
}

function DashboardGridHeader(props: { columns: string; labels: string[] }) {
  return (
    <div className={`grid ${props.columns} rounded-t-md border border-[#e3e9f2] bg-[#f8fafc] px-3 py-3 text-[14px] font-semibold text-slate-600`}>
      {props.labels.map((label) => (
        <div key={label}>{label}</div>
      ))}
    </div>
  );
}

function EmptyDashboardPanel(props: { text: string }) {
  return (
    <div className="flex min-h-[128px] items-center justify-center rounded-md border border-dashed border-[#dbe4ef] bg-[#fbfdff] text-[14px] text-slate-400">
      {props.text}
    </div>
  );
}

function IndexQuoteTile(props: { item: DashboardQuoteState }) {
  const quote = props.item.quote;
  return (
    <div className="min-h-[124px] rounded-md border border-[#e3e9f2] bg-[#fbfdff] px-3 py-3">
      <div className="text-center text-[15px] font-semibold text-slate-700">{indexDisplayName(props.item.symbol)}</div>
      <div className="mt-2 text-center text-[18px] font-semibold text-slate-950">{quote ? formatNumber(quote.price) : "--"}</div>
      <div className={["mt-1 text-center text-[14px] font-semibold", valueToneClass(percentTone(quote?.change_percent))].join(" ")}>
        {formatSignedNumber(quoteChangeAmount(quote))}　{formatPercent(quote?.change_percent)}
      </div>
      <MiniTrendUnavailable />
    </div>
  );
}

function MiniTrendUnavailable() {
  return (
    <div aria-label="指数迷你走势暂未接入" className="mt-2 flex h-[30px] w-full items-center justify-center rounded-sm border border-dashed border-slate-200 bg-slate-50 text-[12px] text-slate-400">
      暂无走势
    </div>
  );
}

function DashboardRiskTips(props: { tips: string[] }) {
  return props.tips.length > 0 ? <div className="mt-4 text-[13px] text-slate-500">{props.tips[0]}</div> : null;
}

const indexNames: Record<string, string> = {
  "000001.SH": "上证指数",
  "399001.SZ": "深证成指",
  "399006.SZ": "创业板指",
  "000300.SH": "沪深300",
};

function indexDisplayName(symbol: string) {
  return indexNames[symbol] || symbol;
}

function latestWatchlistQuoteTime(rows: DashboardWatchlistRow[]) {
  return rows
    .map((row) => row.quote?.quote_time)
    .filter((value): value is string => Boolean(value))
    .sort()
    .at(-1) ?? null;
}

function quoteChangeAmount(quote: MarketQuote | null | undefined) {
  if (!quote) {
    return undefined;
  }
  const legacyQuote = quote as MarketQuote & { change?: number };
  return quote.change_amount ?? legacyQuote.change;
}

function percentTone(value: number | undefined): "up" | "down" | "flat" {
  if (typeof value !== "number" || value === 0) {
    return "flat";
  }
  return value > 0 ? "up" : "down";
}

function valueToneClass(tone: string) {
  if (tone === "up") {
    return "text-red-500";
  }
  if (tone === "down") {
    return "text-green-600";
  }
  return "text-slate-700";
}

function formatNumber(value: number) {
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value);
}

function formatPercent(value: number | undefined) {
  if (typeof value !== "number") {
    return "--";
  }
  return `${value > 0 ? "+" : ""}${value.toFixed(2)}%`;
}

function formatSignedNumber(value: number | undefined) {
  if (typeof value !== "number") {
    return "--";
  }
  return `${value > 0 ? "+" : ""}${formatNumber(value)}`;
}

function formatDateTime(value: string | null | undefined) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

function formatShortDateTime(value: string | undefined) {
  if (!value) {
    return "--";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return `${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")} ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

function marketAmountText(items: DashboardQuoteState[]) {
  const amount = items.reduce((total, item) => total + (item.quote?.amount ?? 0), 0);
  return amount > 0 ? `${formatNumber(amount / 100000000)}亿` : "--";
}

function taskStatusText(status: string) {
  switch (status) {
    case "SUCCESS":
      return "已完成";
    case "RUNNING":
      return "运行中";
    case "FAILED":
      return "失败";
    case "CANCELLED":
      return "已取消";
    default:
      return status || "--";
  }
}
