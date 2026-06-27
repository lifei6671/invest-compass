import { Button, Card, Empty, Progress, Spin, Table, Tag, Tooltip, type TableProps } from "antd";
import { ArrowRightOutlined, FileDoneOutlined, InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { useEffect, type ReactNode } from "react";
import { Link } from "react-router-dom";
import { EChartView } from "../charts/EChartView";
import { useAutoRefresh } from "../../hooks/useAutoRefresh";
import { useDashboardStore, type DashboardQuoteState, type DashboardViewState, type DashboardWatchlistRow } from "../../stores/dashboardStore";
import type { MarketKlineItem, MarketQuote } from "../../services/coreClient";
import { APP_FONT, APP_NUMBER_FONT } from "../../styles/fonts";
import { chinaMarketSessionLabel, chinaMarketSessionStatus, formatClock } from "./dashboardUtils";

export function DashboardOverview() {
  const state = useDashboardStore((store) => store.state);
  const loading = useDashboardStore((store) => store.loading);
  const error = useDashboardStore((store) => store.error);
  const load = useDashboardStore((store) => store.load);
  const hasCachedState = Boolean(state);

  useEffect(() => {
    if (!hasCachedState) {
      void load();
    }
  }, [hasCachedState, load]);
  useAutoRefresh(() => load({ forceRefresh: true }));

  if (error) {
    return <div className="rounded-lg border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-600">总览读取失败：{error}</div>;
  }
  if (!state) {
    return (
      <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-sm text-blue-700">
        正在加载概览缓存数据 {loading ? <Spin className="ml-2" size="small" /> : null}
      </div>
    );
  }

  return (
    <section className="dashboard-overview flex min-h-[calc(100vh-94px)] flex-col gap-4">
      <MarketIndexStrip state={state} />
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[1fr_1fr]">
        <WatchlistDistributionCard state={state} />
        <HotNewsCard state={state} />
      </div>
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[1fr_1fr]">
        <RecentReportsCard state={state} />
        <RecentTasksCard state={state} />
      </div>
      <ProviderStatusCard state={state} />
      <ComplianceBanner tips={state.summary.risk_tips} />
    </section>
  );
}

function MarketIndexStrip(props: { state: DashboardViewState }) {
  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
      {props.state.indexQuotes.map((item) => (
        <IndexQuoteCard key={item.symbol} item={item} trend={props.state.indexTrends[item.symbol] ?? []} />
      ))}
    </div>
  );
}

function IndexQuoteCard(props: { item: DashboardQuoteState; trend: MarketKlineItem[] }) {
  const quote = props.item.quote;
  const tone = percentTone(quote?.change_percent);
  const marketStatus = chinaMarketSessionLabel(chinaMarketSessionStatus(quote?.quote_time));
  const displayName = indexDisplayName(props.item.symbol);
  return (
    <Link aria-label={`查看${displayName}K线图`} className="block text-inherit no-underline" state={{ from: "/" }} to={chartKlinePath(props.item.symbol)}>
      <Card className="dashboard-card dashboard-index-card min-h-[174px] cursor-pointer rounded-lg border-[#dfe7f2] shadow-[0_4px_18px_rgba(15,23,42,0.05)] transition hover:border-[#9fc5ff] hover:shadow-[0_8px_22px_rgba(22,119,255,0.08)]">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="text-[16px] font-medium leading-6 text-slate-950">{displayName}</div>
            <div className="mt-0.5 text-[14px] leading-5 text-slate-500">{props.item.symbol}</div>
            <div className={["app-number mt-4 text-[24px] font-medium leading-8", valueToneClass(tone)].join(" ")}>
              {quote ? formatNumber(quote.price) : "--"}
            </div>
            <div className={["app-number mt-1 flex items-center gap-3 text-[15px] font-medium", valueToneClass(tone)].join(" ")}>
              <span>{formatSignedNumber(quoteChangeAmount(quote))}</span>
              <span>{formatPercent(quote?.change_percent)}</span>
            </div>
          </div>
          <MiniTrendChart items={props.trend} tone={tone} />
        </div>
        <div className="mt-4 text-[14px] text-slate-500">
          {formatClock(quote?.quote_time) || "--:--:--"} <span className="ml-2">{marketStatus}</span>
        </div>
      </Card>
    </Link>
  );
}

function chartKlinePath(symbol: string) {
  return `/chart/kline?symbol=${encodeURIComponent(symbol)}&period=day&adjust=qfq`;
}

function WatchlistDistributionCard(props: { state: DashboardViewState }) {
  const upCount = watchlistCount(props.state.summary.watchlist.up_count);
  const downCount = watchlistCount(props.state.summary.watchlist.down_count);
  const flatCount = watchlistCount(props.state.summary.watchlist.flat_count);
  const total = upCount + downCount + flatCount;
  const rows = [
    { label: "上涨", value: upCount, percent: watchlistPercent(total, upCount), tone: "up" },
    { label: "下跌", value: downCount, percent: watchlistPercent(total, downCount), tone: "down" },
    { label: "平盘", value: flatCount, percent: watchlistPercent(total, flatCount), tone: "flat" },
  ];
  return (
    <DashboardCard className="min-h-[270px]" title="自选股涨跌分布" action={<InfoCircleTooltip title="基于自选股最新行情统计，未读取到行情的股票不计入涨跌分布。" />}>
      <div className="dashboard-watchlist-distribution-body dashboard-watchlist-distribution-stack flex min-h-[190px] flex-wrap items-center gap-x-6 gap-y-5">
        <DistributionDonut rows={rows} total={total} />
        <div className="dashboard-watchlist-distribution-stats flex min-w-[min(100%,360px)] flex-1 flex-wrap items-start gap-x-6 gap-y-4">
          <div className="min-w-[150px] flex-[0_1_170px] space-y-3">
            {rows.map((row) => (
              <div key={row.label} className="grid grid-cols-[48px_28px_minmax(46px,1fr)] items-center gap-1.5 text-[14px]">
                <div className="flex min-w-0 items-center gap-2 text-slate-700">
                  <span className={["h-2.5 w-2.5 shrink-0 rounded-full", dotClass(row.tone)].join(" ")} />
                  <span className="whitespace-nowrap">{row.label}</span>
                </div>
                <div className="app-number text-right text-[16px] font-medium text-slate-950">{row.value}</div>
                <div className="app-number whitespace-nowrap text-right text-slate-500">{row.percent}</div>
              </div>
            ))}
          </div>
          <div className="dashboard-watchlist-distribution-metrics min-w-[170px] flex-1 border-l border-[#e7edf5] px-4">
            <div className="mb-3 flex min-w-0 items-center gap-1 text-[14px] font-medium text-slate-800">
              <span className="truncate">今日总体表现</span>
              <InfoCircleOutlined className="shrink-0 text-[14px] text-slate-400" />
            </div>
            <MetricLine label="平均涨跌幅" value={averageWatchlistChange(props.state.watchlistRows)} tone={percentTone(averageWatchlistChangeNumber(props.state.watchlistRows))} />
            <MetricLine label="上涨概率" value={total > 0 ? `${((upCount / total) * 100).toFixed(2)}%` : "--"} tone="up" />
            <MetricLine label="统计样本" value={total > 0 ? `${total} 只` : "--"} tone="flat" />
          </div>
        </div>
      </div>
    </DashboardCard>
  );
}

function HotNewsCard(props: { state: DashboardViewState }) {
  const newsRows = props.state.summary.market_news.slice(0, 5);
  return (
    <DashboardCard className="h-[270px] overflow-hidden" title="今日热点">
      <MarketNewsList rows={newsRows} />
    </DashboardCard>
  );
}

function RecentReportsCard(props: { state: DashboardViewState }) {
  const rows = props.state.summary.recent_reports.slice(0, 5);
  const columns: TableProps<DashboardRecentReport>["columns"] = [
    { title: "报告标题", dataIndex: "title", width: 210, ellipsis: true, render: (value: string, record) => <span className="block truncate font-medium text-slate-800">{formatReportTitle(value, record)}</span> },
    { title: "关联股票", key: "stock", width: 150, ellipsis: true, render: (_: unknown, record) => <span className="block truncate">{formatReportStock(record)}</span> },
    { title: "分析类型", dataIndex: "analysis_type", width: 100, render: (value?: string) => <AnalysisTypeTag value={value} /> },
    { title: "模型", dataIndex: "model_name", width: 86, ellipsis: true, render: (value?: string) => value || "--" },
    { title: "生成时间", dataIndex: "created_at", width: 112, render: (value?: string) => formatShortDateTime(value) },
    { title: "操作", key: "action", width: 64, render: (_: unknown, record) => <Link className="text-[#1677ff]" to={`/reports/${record.id}`}>查看</Link> },
  ];
  return (
    <DashboardCard className="dashboard-card-fill h-[260px] overflow-hidden" title="最近分析报告">
      {rows.length > 0 ? (
        <div className="dashboard-summary-table-body min-h-0 flex-1 overflow-hidden">
          <Table
            className="dashboard-table"
            rowKey="id"
            size="small"
            tableLayout="fixed"
            pagination={false}
            scroll={{ y: 142 }}
            dataSource={rows}
            columns={columns}
          />
        </div>
      ) : (
        <EmptyDashboardPanel className="flex-1" text="暂无分析报告" />
      )}
      <CardAction label="查看全部报告" to="/reports" />
    </DashboardCard>
  );
}

function RecentTasksCard(props: { state: DashboardViewState }) {
  const rows = props.state.summary.recent_tasks.slice(0, 5);
  const columns: TableProps<DashboardViewState["summary"]["recent_tasks"][number]>["columns"] = [
    { title: "任务标题", dataIndex: "title", width: 180, ellipsis: true, render: (value: string) => <span className="block truncate font-medium text-slate-800">{value}</span> },
    { title: "类型", key: "type", width: 92, ellipsis: true, render: (_: unknown, record) => taskTypeText(record.type ?? record.task_type) },
    { title: "状态", dataIndex: "status", width: 92, render: (value: string) => <TaskStatusTag status={value} /> },
    { title: "进度", dataIndex: "progress", width: 108, render: (value?: number) => <Progress percent={value ?? 0} size="small" showInfo={false} strokeColor="#1677ff" railColor="#e5e7eb" /> },
    { title: "结果摘要", key: "summary", ellipsis: true, render: (_: unknown, record) => <span className="block truncate">{taskResultText(record)}</span> },
  ];
  return (
    <DashboardCard className="dashboard-card-fill h-[260px] overflow-hidden" title="最近任务状态">
      {rows.length > 0 ? (
        <div className="dashboard-summary-table-body min-h-0 flex-1 overflow-hidden">
          <Table<DashboardViewState["summary"]["recent_tasks"][number]>
            className="dashboard-table"
            rowKey="id"
            size="small"
            tableLayout="fixed"
            pagination={false}
            scroll={{ y: 142 }}
            dataSource={rows}
            columns={columns}
          />
        </div>
      ) : (
        <EmptyDashboardPanel className="flex-1" text="暂无任务记录" />
      )}
      <CardAction label="查看全部任务" to="/tasks" />
    </DashboardCard>
  );
}

function ProviderStatusCard(props: { state: DashboardViewState }) {
  const rows = props.state.summary.provider_statuses;
  return (
    <DashboardCard className="overflow-hidden" title="数据源状态">
      {rows.length > 0 ? (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
          {rows.map((row) => (
            <div key={row.name} className="min-w-0 rounded-md border border-[#edf1f7] bg-[#fbfdff] px-4 py-3">
              <div className="flex min-w-0 items-center justify-between gap-3">
                <div className="truncate text-[14px] font-medium text-slate-900">{row.name}</div>
                <Tag className={providerStatusTagClass(row.available)}>{row.available ? "可用" : "异常"}</Tag>
              </div>
              <div className="mt-2 truncate text-[13px] text-slate-500">{row.source || "未提供来源说明"}</div>
              {!row.available && row.last_error ? <div className="mt-1 truncate text-[12px] text-[#ff4d4f]">{row.last_error}</div> : null}
            </div>
          ))}
        </div>
      ) : (
        <EmptyDashboardPanel text="暂无数据源状态" />
      )}
    </DashboardCard>
  );
}

function DashboardCard(props: { title: string; action?: ReactNode; className?: string; children: ReactNode }) {
  return (
    <Card className={["dashboard-card rounded-lg border-[#dfe7f2] shadow-[0_4px_18px_rgba(15,23,42,0.05)]", props.className ?? ""].join(" ")}>
      <div className="mb-3 flex items-center justify-between">
        <div className="text-[16px] font-medium leading-6 text-slate-950">{props.title}</div>
        {props.action}
      </div>
      {props.children}
    </Card>
  );
}

function CardAction(props: { label: string; to: string }) {
  return (
    <div className="dashboard-card-action mt-3 flex h-8 shrink-0 items-center justify-center border-t border-[#eef3f8] pt-3 text-center">
      <Link to={props.to}>
        <Button type="link" className="h-auto px-0 text-[14px] font-medium text-[#1677ff]" iconPlacement="end" icon={<ArrowRightOutlined />}>
          {props.label}
        </Button>
      </Link>
    </div>
  );
}

function InfoCircleTooltip(props: { title: string }) {
  return (
    <Tooltip title={props.title}>
      <Button aria-label="说明" type="text" className="h-6 w-6 p-0 text-slate-400" icon={<InfoCircleOutlined />} />
    </Tooltip>
  );
}

function MarketNewsList(props: { rows: DashboardViewState["summary"]["market_news"] }) {
  if (props.rows.length === 0) {
    return <EmptyDashboardPanel text="暂无市场新闻" />;
  }
  return (
    <div className="space-y-3 pt-1">
      {props.rows.map((row, index) => (
        <div key={`${row.title}-${index}`} className="grid grid-cols-[24px_1fr_auto] items-center gap-3 text-[14px]">
          <span className={["flex h-5 w-5 items-center justify-center rounded-[5px] app-number text-[13px] font-medium", rankClass(index)].join(" ")}>
            {index + 1}
          </span>
          <span className="truncate font-medium text-slate-800">{row.title}</span>
          <Tag className="m-0 rounded-md border-0 bg-[#f1f5f9] text-slate-500">{row.source || "市场新闻"}</Tag>
        </div>
      ))}
    </div>
  );
}

function UnavailablePanel(props: { text: string }) {
  return <div className="flex min-h-[154px] items-center justify-center rounded-md border border-dashed border-[#dbe4ef] bg-[#fbfdff] text-[14px] text-slate-400">{props.text}</div>;
}

function EmptyDashboardPanel(props: { text: string; className?: string }) {
  return (
    <div className={["flex min-h-[150px] items-center justify-center rounded-md border border-dashed border-[#dbe4ef] bg-[#fbfdff]", props.className ?? ""].join(" ")}>
      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<span className="text-slate-400">{props.text}</span>} />
    </div>
  );
}

function DistributionDonut(props: { rows: Array<{ label: string; value: number; tone: string }>; total: number }) {
  if (props.total === 0 || isJSDOM()) {
    return (
      <div className="flex h-[126px] w-[126px] items-center justify-center rounded-full border-[16px] border-slate-100 text-center">
        <div>
          <div className="text-[14px] text-slate-500">总数</div>
          <div className="app-number text-[24px] font-medium text-slate-950">{props.total}</div>
        </div>
      </div>
    );
  }
  const option = {
    animation: false,
    textStyle: {
      fontFamily: APP_FONT,
      fontSize: 14,
      fontWeight: 400,
    },
    series: [
      {
        type: "pie",
        radius: ["58%", "78%"],
        center: ["50%", "50%"],
        avoidLabelOverlap: true,
        label: { show: false },
        labelLine: { show: false },
        data: props.rows.map((row) => ({ name: row.label, value: row.value, itemStyle: { color: chartColor(row.tone) } })),
      },
    ],
    graphic: [
      { type: "text", left: "center", top: 46, style: { text: "总数", fill: "#64748b", font: `400 14px ${APP_FONT}`, textAlign: "center" } },
      { type: "text", left: "center", top: 66, style: { text: String(props.total), fill: "#0f172a", font: `500 24px ${APP_NUMBER_FONT}`, textAlign: "center" } },
    ],
  };
  return <EChartView option={option} style={{ height: 126, width: 126 }} />;
}

function MiniTrendChart(props: { items: MarketKlineItem[]; tone: "up" | "down" | "flat" }) {
  const points = props.items.map((item) => item.close).filter((value) => typeof value === "number");
  if (points.length < 2 || isJSDOM()) {
    return <div className="mt-12 flex h-[52px] w-[132px] items-center justify-center rounded-md bg-slate-50 text-[14px] text-slate-400">暂无走势</div>;
  }
  const color = chartColor(props.tone);
  const option = {
    animation: false,
    textStyle: {
      fontFamily: APP_FONT,
      fontSize: 13,
      fontWeight: 400,
    },
    grid: { left: 0, right: 0, top: 4, bottom: 4 },
    xAxis: { type: "category", show: false, boundaryGap: false, data: props.items.map((item) => item.trade_date) },
    yAxis: { type: "value", show: false, scale: true },
    series: [
      {
        type: "line",
        data: points,
        showSymbol: false,
        smooth: true,
        lineStyle: { width: 2, color },
        areaStyle: { color: "rgba(22,119,255,0.04)" },
      },
    ],
  };
  return <EChartView aria-label="指数迷你走势" option={option} style={{ height: 52, width: 132 }} />;
}

function ComplianceBanner(props: { tips: string[] }) {
  const mainTip = props.tips[0] || "AI 生成内容仅供研究参考，请结合公开披露信息独立判断。";
  return (
    <div className="mt-auto flex min-h-[52px] items-center justify-between rounded-lg border border-[#b9d6ff] bg-[#f2f7ff] px-6 text-[14px] text-[#155ec8]">
      <div className="flex items-center gap-3 font-medium">
        <InfoCircleOutlined className="text-[18px]" />
        {mainTip}
      </div>
      <div className="flex items-center gap-2 text-slate-500">
        <SafetyCertificateOutlined />
        仅供研究，不构成投资建议。
      </div>
    </div>
  );
}

function MetricLine(props: { label: string; value: string; tone: string }) {
  return (
    <div className="mb-3 last:mb-0">
      <div className="text-[14px] text-slate-500">{props.label}</div>
      <div className={["app-number mt-1 text-[17px] font-medium", valueToneClass(props.tone)].join(" ")}>{props.value}</div>
    </div>
  );
}

type DashboardRecentReport = DashboardViewState["summary"]["recent_reports"][number];

function formatReportStock(report: DashboardRecentReport) {
  const symbol = report.symbol?.trim();
  const stockName = report.stock_name?.trim();
  if (stockName && symbol) {
    return `${stockName}（${symbol}）`;
  }
  return stockName || symbol || "--";
}

function formatReportTitle(title: string | undefined, report: DashboardRecentReport) {
  const rawTitle = title?.trim() || "";
  const analysisType = analysisTypeLabel(report.analysis_type);
  const stockLabel = report.stock_name?.trim() || report.symbol?.trim();
  if (/stock_full|technical|fundamental|news|custom/.test(rawTitle) && stockLabel) {
    return `${stockLabel} ${analysisType}报告`;
  }
  return rawTitle.replace(/\b(stock_full|technical|fundamental|news|custom)\b/g, (value) => analysisTypeLabel(value)) || "--";
}

function analysisTypeLabel(value?: string) {
  switch ((value || "").trim()) {
    case "stock_full":
      return "个股综合分析";
    case "technical":
      return "技术面分析";
    case "fundamental":
      return "基本面分析";
    case "news":
      return "消息面分析";
    case "custom":
      return "自定义分析";
    default:
      return value?.trim() || "个股综合分析";
  }
}

function AnalysisTypeTag(props: { value?: string }) {
  const value = analysisTypeLabel(props.value);
  const tone = value.includes("基本") ? "green" : value.includes("技术") ? "orange" : "blue";
  const className =
    tone === "green"
      ? "m-0 rounded-md border-0 bg-[#e9f8ef] text-[#18a058]"
      : tone === "orange"
        ? "m-0 rounded-md border-0 bg-[#fff2e6] text-[#f97316]"
        : "m-0 rounded-md border-0 bg-[#eaf2ff] text-[#1677ff]";
  return <Tag className={className}>{value}</Tag>;
}

function TaskStatusTag(props: { status: string }) {
  const color = props.status === "RUNNING" ? "processing" : props.status === "SUCCESS" ? "success" : props.status === "FAILED" ? "error" : "default";
  return <Tag color={color}>{props.status}</Tag>;
}

function providerStatusTagClass(available: boolean) {
  return available ? "m-0 rounded-md border-0 bg-[#e9f8ef] text-[#16a34a]" : "m-0 rounded-md border-0 bg-[#fff1f0] text-[#ff4d4f]";
}

function watchlistCount(value: number) {
  return Number.isFinite(value) ? value : 0;
}

function watchlistPercent(total: number, value: number) {
  if (total === 0) {
    return "--";
  }
  return `${((value / total) * 100).toFixed(2)}%`;
}

function averageWatchlistChangeNumber(rows: DashboardWatchlistRow[]) {
  const values = rows.map((row) => row.quote?.change_percent).filter((value): value is number => typeof value === "number");
  if (values.length === 0) {
    return undefined;
  }
  return values.reduce((total, value) => total + value, 0) / values.length;
}

function averageWatchlistChange(rows: DashboardWatchlistRow[]) {
  const value = averageWatchlistChangeNumber(rows);
  return typeof value === "number" ? formatPercent(value) : "--";
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

function dotClass(tone: string) {
  if (tone === "up") {
    return "bg-red-500";
  }
  if (tone === "down") {
    return "bg-green-500";
  }
  return "bg-slate-300";
}

function chartColor(tone: string) {
  if (tone === "up") {
    return "#f43f5e";
  }
  if (tone === "down") {
    return "#22c55e";
  }
  return "#cbd5e1";
}

function rankClass(index: number) {
  if (index === 0) {
    return "bg-red-500 text-white";
  }
  if (index === 1 || index === 2) {
    return "bg-orange-400 text-white";
  }
  return "bg-slate-100 text-slate-500";
}

function indexDisplayName(symbol: string) {
  return indexNames[symbol] || symbol;
}

const indexNames: Record<string, string> = {
  "000001.SH": "上证指数",
  "399001.SZ": "深证成指",
  "399006.SZ": "创业板指",
  "000300.SH": "沪深300",
};

function taskTypeText(value?: string) {
  switch (value) {
    case "ANALYSIS":
      return "AI 分析报告";
    case "MARKET":
      return "市场分析";
    case "SEARCH_REBUILD":
      return "索引重建";
    default:
      return value || "--";
  }
}

function taskResultText(record: { status: string; error_message?: string; progress?: number }) {
  if (record.status === "FAILED") {
    return record.error_message || "任务失败";
  }
  if (record.status === "RUNNING") {
    return "正在生成报告中...";
  }
  if (record.status === "SUCCESS") {
    return "已完成";
  }
  return `${record.progress ?? 0}%`;
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

function isJSDOM() {
  return typeof navigator !== "undefined" && navigator.userAgent.toLowerCase().includes("jsdom");
}
