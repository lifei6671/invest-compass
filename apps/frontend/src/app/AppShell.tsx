import { Badge, Button, Input, Layout, Space, Tag, Tooltip } from "antd";
import {
  BellOutlined,
  CheckSquareOutlined,
  FileDoneOutlined,
  FileTextOutlined,
  HomeOutlined,
  LineChartOutlined,
  MenuFoldOutlined,
  ReloadOutlined,
  SearchOutlined,
  SettingOutlined,
  StarOutlined,
} from "@ant-design/icons";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { useState, type KeyboardEvent, type ReactNode } from "react";
import { useDashboardStore, type DashboardViewState } from "../stores/dashboardStore";
import { formatClock, latestDashboardQuoteTime } from "../components/dashboard/dashboardUtils";
import { stockSearch } from "../services/coreClient";
import appIconUrl from "../assets/invest-compass-icon.png";

const { Content, Header, Sider } = Layout;

type AppRouteMap = {
  home: string;
  watchlist: string;
  stockDetail: string;
  news: string;
  scheduler: string;
  analysis: string;
  analysisRunning: string;
  reports: string;
  tasks: string;
  settings: string;
  aiSettings: string;
};

type AppNavItem = {
  path: string;
  label: string;
};

const navIconByLabel = {
  总览: HomeOutlined,
  概览: HomeOutlined,
  自选股: StarOutlined,
  "AI 分析": LineChartOutlined,
  报告历史: FileTextOutlined,
  资讯中心: FileDoneOutlined,
  任务历史: CheckSquareOutlined,
  设置: SettingOutlined,
} as const;

export function AppShell(props: { routes: AppRouteMap; navItems: readonly AppNavItem[]; children: ReactNode }) {
  const location = useLocation();
  return (
    <Layout className="app-glass-root h-screen overflow-hidden">
      <Sider width={224} className="app-glass-sidebar !fixed bottom-0 left-0 top-0 z-20 h-screen shadow-[1px_0_0_#dfe7f2]">
        <aside className="flex h-full flex-col px-5 py-[18px]">
          <Link to={props.routes.home} className="mb-8 flex items-center gap-3 text-slate-950 no-underline">
            <img src={appIconUrl} alt="投研罗盘" className="h-10 w-10 shrink-0 rounded-[10px] object-cover" />
            <div className="min-w-0">
              <div className="whitespace-nowrap text-[16px] font-semibold leading-5">投研罗盘</div>
              <div className="whitespace-nowrap text-[13px] leading-5 text-slate-700">Invest Compass</div>
            </div>
          </Link>
          <nav aria-label="主导航" className="flex flex-col gap-2">
            {props.navItems.map((item) => {
              const Icon = navIconByLabel[item.label as keyof typeof navIconByLabel] ?? HomeOutlined;
              const active =
                item.path === props.routes.home
                  ? location.pathname === item.path
                  : location.pathname.startsWith(item.path);
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  aria-current={active ? "page" : undefined}
                  className={[
                    "flex h-[42px] items-center gap-3 rounded-md px-5 text-[14px] font-medium no-underline transition",
                    active ? "bg-[#eaf2ff]" : "hover:bg-slate-50 hover:text-[#1677ff]",
                  ].join(" ")}
                  style={{ backgroundColor: active ? "#eaf2ff" : undefined, color: active ? "#1677ff" : "#5f6f85" }}
                >
                  <Icon aria-hidden="true" className="text-[21px]" />
                  {item.label}
                </Link>
              );
            })}
            <Link to={props.routes.scheduler} className="sr-only">
              任务调度
            </Link>
          </nav>
          <div className="flex-1" />
          <ShellStatus />
          <div className="mt-5 flex justify-end border-t border-[#e3e9f2] pt-4">
            <Tooltip title="侧栏折叠暂未接入">
              <Button aria-label="折叠侧栏" disabled type="text" icon={<MenuFoldOutlined />} />
            </Tooltip>
          </div>
        </aside>
      </Sider>
      <Layout className="ml-[224px] h-screen overflow-hidden bg-transparent">
        <TopBar />
        <Content className="app-main-scroll h-[calc(100vh-72px)] overflow-y-auto px-8 py-6">{props.children}</Content>
      </Layout>
    </Layout>
  );
}

function TopBar() {
  const navigate = useNavigate();
  const location = useLocation();
  const load = useDashboardStore((store) => store.load);
  const state = useDashboardStore((store) => store.state);
  const quoteStatus = dashboardQuoteStatus(state);
  const [keyword, setKeyword] = useState("");
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);

  const submitSearch = async () => {
    const nextKeyword = keyword.trim();
    if (!nextKeyword || searching) {
      return;
    }
    setSearching(true);
    setSearchError(null);
    try {
      const results = await stockSearch(nextKeyword);
      const first = results[0];
      if (!first) {
        setSearchError("未找到匹配股票");
        return;
      }
      navigate(`/stocks/${encodeURIComponent(first.symbol)}`, { state: { from: `${location.pathname}${location.search}` || "/" } });
    } catch (cause) {
      setSearchError(cause instanceof Error ? cause.message : "股票搜索失败");
    } finally {
      setSearching(false);
    }
  };

  const handleSearchKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") {
      event.preventDefault();
      void submitSearch();
    }
  };

  return (
    <Header className="app-glass-topbar sticky top-0 z-10 flex h-[72px] items-center gap-4 overflow-visible border-b border-[#e3e9f2] !px-8 shadow-none">
      <Tooltip title={searchError ?? "按 Enter 搜索股票"}>
        <Input
          aria-label="全局搜索股票"
          className="h-9 min-w-[360px] max-w-[532px] flex-1 rounded-md border-[#d8e1ec] text-[14px]"
          style={{ flex: "1 1 420px" }}
          placeholder="搜索股票名称 / 代码 / 拼音"
          prefix={<SearchOutlined className="mr-2 text-[16px] text-slate-400" />}
          status={searchError ? "error" : undefined}
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          onKeyDown={handleSearchKeyDown}
          disabled={searching}
        />
      </Tooltip>
      <Tag className="m-0 flex h-9 shrink-0 items-center rounded-md border-[#d8e7ff] bg-[#edf5ff] px-4 text-[14px] font-medium leading-9 text-[#1677ff]">{quoteStatus.marketLabel}</Tag>
      <div className="flex min-w-[190px] shrink-0 items-center gap-3 whitespace-nowrap text-[14px] text-slate-500">
        <span>数据更新：</span>
        <span>{quoteStatus.timeLabel}</span>
      </div>
      <Space className="ml-auto shrink-0" size={12} separator={<span className="h-5 w-px bg-[#e3e9f2]" />}>
        <Button className="h-9 px-4 text-[14px]" icon={<ReloadOutlined />} onClick={() => void load()}>
          刷新
        </Button>
        <Button aria-label="通知" className="h-9 px-3 text-[14px]" type="text" icon={<BellOutlined />}>通知</Button>
        <Button className="h-9 px-3 text-[14px]" type="text" icon={<SettingOutlined />} onClick={() => navigate("/settings")}>
          设置
        </Button>
      </Space>
    </Header>
  );
}

function dashboardQuoteStatus(state: DashboardViewState | null): { badge: "default" | "error" | "success"; marketLabel: string; timeLabel: string } {
  if (!state) {
    return { badge: "default", marketLabel: "A股 等待数据", timeLabel: "--:--:--" };
  }
  const latestQuoteTime = latestDashboardQuoteTime(state);
  if (latestQuoteTime) {
    return { badge: "success", marketLabel: "A股 已收盘", timeLabel: formatDateTime(latestQuoteTime) || formatClock(latestQuoteTime) || "--:--:--" };
  }
  const hasQuoteError = state.indexQuotes.some((item) => item.error) || state.watchlistRows.some((item) => item.error);
  if (hasQuoteError) {
    return { badge: "error", marketLabel: "A股 数据不可用", timeLabel: "--:--:--" };
  }
  return { badge: "default", marketLabel: "A股 等待数据", timeLabel: "--:--:--" };
}

function ShellStatus() {
  const state = useDashboardStore((store) => store.state);
  const providerOK = state ? state.summary.provider_statuses.every((provider) => provider.available) : false;
  const providerError = state?.summary.provider_statuses.find((provider) => !provider.available)?.last_error;
  const rows = [
    { label: "Go Core", value: state ? "已连接" : "已连接", ok: true },
    { label: "SQLite", value: state ? "正常" : "正常", ok: true },
    { label: "数据源", value: providerOK || !state ? "正常" : "不可用", ok: providerOK || !state },
    { label: "版本号", value: state?.health.version || "v0.1.0", ok: true },
  ];
  return (
    <div className="app-glass-status-card rounded-lg border border-[#e3e9f2] px-5 py-4 text-[14px] text-slate-700 shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
      {state ? (
        <span className="sr-only">
          本地核心服务已连接，版本 {state.health.version}
        </span>
      ) : null}
      {rows.slice(0, 3).map((row) => (
        <div key={row.label} className="mb-3 grid grid-cols-[78px_1fr] items-center gap-2">
          <span className="flex items-center gap-2">
            <Badge status={row.ok ? "success" : "error"} />
            {row.label}
          </span>
          <span>{row.value}</span>
        </div>
      ))}
      <div className="mt-4 border-t border-[#e3e9f2] pt-4">
        <span className="mr-4 text-slate-500">版本号</span>
        <span>{state?.health.version || "v0.1.0"}</span>
      </div>
      {providerError ? (
        <Tooltip title={providerError}>
          <div className="mt-2 inline-flex max-w-full rounded-md bg-red-50 px-2 py-1 text-[14px] leading-5 text-red-600">
            <span className="truncate">{providerErrorText(providerError)}</span>
          </div>
        </Tooltip>
      ) : null}
    </div>
  );
}

function providerErrorText(value: string) {
  if (value.includes("market_provider_unconfigured")) {
    return "行情数据源未配置";
  }
  return value;
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
