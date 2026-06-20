import { Badge, Button, Input, Layout, Space, Tag, Tooltip } from "antd";
import {
  BellOutlined,
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
import { formatClock, latestDashboardQuoteTime } from "../components/dashboard/DashboardOverview";
import { stockSearch } from "../services/coreClient";

const { Content, Header, Sider } = Layout;

type AppRouteMap = {
  home: string;
  watchlist: string;
  stockDetail: string;
  news: string;
  scheduler: string;
  analysis: string;
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
  概览: HomeOutlined,
  自选股: StarOutlined,
  "AI 分析": LineChartOutlined,
  报告历史: FileTextOutlined,
  资讯中心: FileDoneOutlined,
  任务历史: MenuFoldOutlined,
  设置: SettingOutlined,
} as const;

export function AppShell(props: { routes: AppRouteMap; navItems: readonly AppNavItem[]; children: ReactNode }) {
  const location = useLocation();
  return (
    <Layout className="min-h-screen overflow-hidden bg-[#f7faff]">
      <Sider width={246} className="!fixed bottom-0 left-0 top-0 z-20 !bg-white shadow-[1px_0_0_#dfe7f2]">
        <aside className="flex h-full flex-col px-3 py-6">
          <Link to={props.routes.home} className="mb-8 flex items-center gap-3 px-5 text-slate-950 no-underline">
            <div className="flex h-12 w-12 items-center justify-center rounded-full border-2 border-[#1677ff] text-[#1677ff]">
              <LineChartOutlined aria-hidden="true" className="text-[26px]" />
            </div>
            <div>
              <div className="text-[23px] font-bold leading-7 tracking-wide">投研罗盘</div>
              <div className="text-[15px] leading-5 text-slate-600">Invest Compass</div>
            </div>
          </Link>
          <nav aria-label="主导航" className="flex flex-1 flex-col gap-4">
            {props.navItems.map((item) => {
              const Icon = navIconByLabel[item.label as keyof typeof navIconByLabel] ?? HomeOutlined;
              const active = item.path === props.routes.home ? location.pathname === item.path : location.pathname.startsWith(item.path);
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  className={[
                    "flex h-[58px] items-center gap-4 rounded-lg px-5 text-[16px] font-medium no-underline transition",
                    active ? "bg-[#eaf2ff] text-[#1677ff]" : "text-slate-700 hover:bg-slate-50 hover:text-[#1677ff]",
                  ].join(" ")}
                >
                  <Icon aria-hidden="true" className="text-[25px]" />
                  {item.label}
                </Link>
              );
            })}
            <Link to={props.routes.scheduler} className="sr-only">
              任务调度
            </Link>
          </nav>
          <ShellStatus />
        </aside>
      </Sider>
      <Layout className="ml-[246px] min-h-screen bg-transparent">
        <TopBar />
        <Content className="min-h-[calc(100vh-92px)] px-[34px] py-[28px]">{props.children}</Content>
      </Layout>
    </Layout>
  );
}

function TopBar() {
  const navigate = useNavigate();
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
      navigate(`/stocks/${encodeURIComponent(first.symbol)}`);
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
    <Header className="sticky top-0 z-10 flex h-[92px] items-center gap-7 border-b border-[#e3e9f2] !bg-white px-[40px] shadow-none">
      <Tooltip title={searchError ?? "按 Enter 搜索股票"}>
        <Input
          aria-label="全局搜索股票"
          className="h-[52px] max-w-[510px] rounded-lg border-[#d8e1ec] text-[15px]"
          placeholder="搜索股票名称 / 代码 / 拼音"
          prefix={<SearchOutlined className="mr-2 text-[19px] text-slate-400" />}
          status={searchError ? "error" : undefined}
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          onKeyDown={handleSearchKeyDown}
          disabled={searching}
        />
      </Tooltip>
      <Tag className="rounded-md border-[#d8e7ff] bg-[#edf5ff] px-4 py-1.5 text-[15px] font-semibold text-[#1677ff]">A股市场</Tag>
      <div className="flex min-w-[220px] flex-1 items-center gap-3 whitespace-nowrap text-[15px] text-slate-700">
        <Badge status={quoteStatus.badge} />
        <span>{quoteStatus.label}</span>
        <span>{quoteStatus.timeLabel}</span>
      </div>
      <Space size={26}>
        <Button className="h-[50px] px-6 text-[15px]" icon={<ReloadOutlined />} onClick={() => void load()}>
          刷新
        </Button>
        <Tooltip title="通知中心">
          <Button aria-label="通知中心" className="h-[50px] w-[56px]" disabled icon={<BellOutlined />} />
        </Tooltip>
        <Button className="h-[50px] px-6 text-[15px]" icon={<SettingOutlined />} onClick={() => navigate("/settings")}>
          设置
        </Button>
      </Space>
    </Header>
  );
}

function dashboardQuoteStatus(state: DashboardViewState | null): { badge: "default" | "error" | "success"; label: string; timeLabel: string } {
  if (!state) {
    return { badge: "default", label: "等待市场数据", timeLabel: "--:--:--" };
  }
  const latestQuoteTime = latestDashboardQuoteTime(state);
  if (latestQuoteTime) {
    return { badge: "success", label: "市场数据已更新", timeLabel: formatClock(latestQuoteTime) || "--:--:--" };
  }
  const hasQuoteError = state.indexQuotes.some((item) => item.error) || state.watchlistRows.some((item) => item.error);
  if (hasQuoteError) {
    return { badge: "error", label: "市场数据暂不可用", timeLabel: "--:--:--" };
  }
  return { badge: "default", label: "等待市场数据", timeLabel: "--:--:--" };
}

function ShellStatus() {
  const state = useDashboardStore((store) => store.state);
  const providerOK = state ? state.summary.provider_statuses.every((provider) => provider.available) : false;
  const providerError = state?.summary.provider_statuses.find((provider) => !provider.available)?.last_error;
  const rows = [
    { label: "Go Core", value: state ? "已连接" : "未连接", ok: Boolean(state) },
    { label: "SQLite", value: state ? "正常" : "未知", ok: Boolean(state) },
    { label: "数据源", value: providerOK ? "正常" : "不可用", ok: providerOK },
    { label: "版本号", value: state?.health.version || "v0.1.0", ok: true },
  ];
  return (
    <div className="border-t border-[#dfe7f2] px-3 pt-5 text-[13px] text-slate-700">
      {state ? (
        <span className="sr-only">
          本地核心服务已连接，版本 {state.health.version}
        </span>
      ) : null}
      {rows.map((row) => (
        <div key={row.label} className="mb-3 grid grid-cols-[68px_1fr] items-center gap-2">
          <span>{row.label}:</span>
          <span className="flex items-center gap-2">
            {row.label !== "版本号" ? <Badge status={row.ok ? "success" : "error"} /> : null}
            {row.value}
          </span>
        </div>
      ))}
      {providerError ? <div className="mt-1 break-all text-xs text-red-500">{providerError}</div> : null}
    </div>
  );
}
