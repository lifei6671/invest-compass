import { App as AntdApp, Badge, Button, Empty, Input, Layout, Popover, Space, Spin, Tag, Tooltip } from "antd";
import {
  BarChartOutlined,
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
import { useCallback, useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { useDashboardStore, type DashboardViewState } from "../stores/dashboardStore";
import { formatClock, latestDashboardQuoteTime } from "../components/dashboard/dashboardUtils";
import {
  coreHealth,
  notificationsClearRead,
  notificationsList,
  notificationsMarkAllRead,
  notificationsMarkRead,
  notificationsUnreadCount,
  providersStatus,
  settingsGet,
  stockSearch,
  type CoreHealth,
  type NotificationItem,
  type ProviderStatusItem,
} from "../services/coreClient";
import {
  sendConfiguredDesktopNotification,
  type DesktopNotificationKind,
  type DesktopNotificationSettings,
} from "../services/desktopNotification";
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
  个股详情: BarChartOutlined,
  "AI 分析": LineChartOutlined,
  报告历史: FileTextOutlined,
  资讯中心: FileDoneOutlined,
  任务历史: CheckSquareOutlined,
  设置: SettingOutlined,
} as const;

const notificationPollIntervalMs = 30_000;
const lockedActionMessage = "系统初始化中，请稍候";
const initializationNavItems = [
  { path: "/", label: "总览" },
  { path: "/watchlist", label: "自选股" },
  { path: "/stocks", label: "个股详情" },
  { path: "/analysis", label: "AI 分析" },
  { path: "/reports", label: "报告历史" },
  { path: "/news", label: "资讯中心" },
  { path: "/tasks", label: "任务历史" },
  { path: "/settings", label: "设置" },
] as const;

const desktopNotificationSettingKeys = [
  "notifications.system_enabled",
  "notifications.task_success",
  "notifications.task_failed",
  "notifications.provider_error",
] as const;

export function AppShell(props: { routes: AppRouteMap; navItems: readonly AppNavItem[]; children: ReactNode; locked?: boolean }) {
  const location = useLocation();
  const { message } = AntdApp.useApp();
  const locked = Boolean(props.locked);
  const navItems = locked ? initializationNavItems : props.navItems;
  const showLockedMessage = () => {
    void message.info(lockedActionMessage);
  };
  return (
    <Layout className="app-glass-root h-screen overflow-hidden">
      <Sider width={224} className="app-glass-sidebar !fixed bottom-0 left-0 top-0 z-20 h-screen shadow-[1px_0_0_#dfe7f2]">
        <aside className="flex h-full flex-col px-5 py-[18px]">
          {locked ? (
            <div className="mb-8 flex items-center gap-3 text-slate-950">
              <img src={appIconUrl} alt="投研罗盘" className="h-10 w-10 shrink-0 rounded-[10px] object-cover" />
              <div className="min-w-0">
                <div className="whitespace-nowrap text-[16px] font-semibold leading-5">投研罗盘</div>
                <div className="whitespace-nowrap text-[13px] leading-5 text-slate-700">Invest Compass</div>
              </div>
            </div>
          ) : (
            <Link to={props.routes.home} className="mb-8 flex items-center gap-3 text-slate-950 no-underline">
            <img src={appIconUrl} alt="投研罗盘" className="h-10 w-10 shrink-0 rounded-[10px] object-cover" />
            <div className="min-w-0">
              <div className="whitespace-nowrap text-[16px] font-semibold leading-5">投研罗盘</div>
              <div className="whitespace-nowrap text-[13px] leading-5 text-slate-700">Invest Compass</div>
            </div>
            </Link>
          )}
          <nav aria-label="主导航" className="flex flex-col gap-2">
            {navItems.map((item) => {
              const Icon = navIconByLabel[item.label as keyof typeof navIconByLabel] ?? HomeOutlined;
              const active =
                !locked &&
                (item.path === props.routes.home
                  ? location.pathname === item.path
                  : location.pathname.startsWith(item.path));
              if (locked) {
                return (
                  <button
                    key={item.path}
                    type="button"
                    aria-disabled="true"
                    className="flex h-[42px] items-center gap-3 rounded-md border-0 bg-transparent px-5 text-left text-[14px] font-medium text-[#9ca3af] transition hover:bg-[#f8fafc]"
                    onClick={showLockedMessage}
                  >
                    <Icon aria-hidden="true" className="text-[21px]" />
                    {item.label}
                  </button>
                );
              }
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
            {locked ? null : (
              <Link to={props.routes.scheduler} className="sr-only">
                任务调度
              </Link>
            )}
          </nav>
          <div className="flex-1" />
          <ShellStatus locked={locked} />
          <div className="mt-5 flex justify-end border-t border-[#e3e9f2] pt-4">
            <Tooltip title="侧栏折叠暂未接入">
              <Button aria-label="折叠侧栏" disabled type="text" icon={<MenuFoldOutlined />} />
            </Tooltip>
          </div>
        </aside>
      </Sider>
      <Layout className="ml-[224px] h-screen overflow-hidden bg-transparent">
        <TopBar locked={locked} />
        <Content className="app-main-scroll h-[calc(100vh-72px)] overflow-y-auto px-8 py-6">{props.children}</Content>
      </Layout>
    </Layout>
  );
}

function TopBar(props: { locked?: boolean }) {
  const navigate = useNavigate();
  const location = useLocation();
  const { message } = AntdApp.useApp();
  const locked = Boolean(props.locked);
  const load = useDashboardStore((store) => store.load);
  const state = useDashboardStore((store) => store.state);
  const quoteStatus = locked ? { badge: "success" as const, marketLabel: "A股 已收盘", timeLabel: "2025-05-22 15:29:45" } : dashboardQuoteStatus(state);
  const [keyword, setKeyword] = useState("");
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState<string | null>(null);
  const [notificationOpen, setNotificationOpen] = useState(false);
  const [notificationLoading, setNotificationLoading] = useState(false);
  const [notificationItems, setNotificationItems] = useState<NotificationItem[]>([]);
  const [notificationUnreadCount, setNotificationUnreadCount] = useState(0);
  const previousUnreadCountRef = useRef<number | null>(null);
  const desktopNotifiedIdsRef = useRef<Set<number>>(new Set());

  const sendUnreadDesktopNotifications = useCallback(async () => {
    const result = await notificationsList({ unread_only: true, limit: 5, offset: 0 });
    const settings = await loadDesktopNotificationSettings();
    for (const item of result.items) {
      if (item.is_read || desktopNotifiedIdsRef.current.has(item.id)) {
        continue;
      }
      const kind = notificationKindFromItem(item);
      if (!kind) {
        continue;
      }
      const delivery = await sendConfiguredDesktopNotification(
        { kind, title: item.title, body: item.content || item.title },
        settings,
      );
      if (delivery.sent || delivery.reason !== "failed") {
        desktopNotifiedIdsRef.current.add(item.id);
      }
    }
  }, []);

  const loadNotificationUnreadCount = useCallback(async (options?: { notifyDesktop?: boolean }) => {
    const result = await notificationsUnreadCount();
    const nextCount = Number.isFinite(result.count) ? Math.max(0, result.count) : 0;
    const previousCount = previousUnreadCountRef.current;
    previousUnreadCountRef.current = nextCount;
    setNotificationUnreadCount(nextCount);
    if (options?.notifyDesktop && previousCount !== null && nextCount > previousCount) {
      await sendUnreadDesktopNotifications();
    }
  }, [sendUnreadDesktopNotifications]);

  const loadNotifications = useCallback(async () => {
    setNotificationLoading(true);
    try {
      const result = await notificationsList({ unread_only: false, limit: 20, offset: 0 });
      setNotificationItems(result.items);
      await loadNotificationUnreadCount();
    } finally {
      setNotificationLoading(false);
    }
  }, [loadNotificationUnreadCount]);

  useEffect(() => {
    if (locked) {
      return;
    }
    loadNotificationUnreadCount()
      .catch(() => {
        previousUnreadCountRef.current = 0;
        setNotificationUnreadCount(0);
      });
    const timer = window.setInterval(() => {
      void loadNotificationUnreadCount({ notifyDesktop: true }).catch(() => {
        previousUnreadCountRef.current = 0;
        setNotificationUnreadCount(0);
      });
    }, notificationPollIntervalMs);
    return () => {
      window.clearInterval(timer);
    };
  }, [loadNotificationUnreadCount, locked]);

  const showLockedMessage = () => {
    void message.info(lockedActionMessage);
  };

  const handleNotificationOpenChange = (open: boolean) => {
    setNotificationOpen(open);
    if (open) {
      void loadNotifications().catch(() => {
        setNotificationItems([]);
        setNotificationUnreadCount(0);
      });
    }
  };

  const markNotificationRead = async (item: NotificationItem) => {
    if (item.is_read) {
      return;
    }
    await notificationsMarkRead({ ids: [item.id] });
    setNotificationItems((items) =>
      items.map((current) =>
        current.id === item.id
          ? { ...current, is_read: true, read_at: current.read_at ?? new Date().toISOString() }
          : current,
      ),
    );
    setNotificationUnreadCount((count) => Math.max(0, count - 1));
  };

  const handleNotificationClick = async (item: NotificationItem) => {
    try {
      await markNotificationRead(item);
      const route = resolveNotificationRoute(item.route);
      if (route) {
        setNotificationOpen(false);
        navigate(route);
      }
    } catch {
      // 标记已读失败时保留当前浮层，避免界面显示与后端状态不一致。
    }
  };

  const handleMarkAllNotificationsRead = async () => {
    await notificationsMarkAllRead();
    setNotificationItems((items) =>
      items.map((item) => ({ ...item, is_read: true, read_at: item.read_at ?? new Date().toISOString() })),
    );
    setNotificationUnreadCount(0);
  };

  const handleClearReadNotifications = async () => {
    await notificationsClearRead();
    setNotificationItems((items) => items.filter((item) => !item.is_read));
    await loadNotificationUnreadCount();
  };

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

  const notificationContent = (
    <div className="w-[360px] overflow-hidden rounded-[10px] bg-white">
      <div className="flex items-center justify-between border-b border-[#edf1f7] px-1 pb-3">
        <div className="text-[15px] font-semibold text-[#111827]">通知</div>
        <Space size={8}>
          <Button
            className="h-7 px-2 text-[12px]"
            disabled={notificationUnreadCount === 0}
            type="link"
            onClick={() => void handleMarkAllNotificationsRead()}
          >
            全部已读
          </Button>
          <Button className="h-7 px-2 text-[12px]" type="link" onClick={() => void handleClearReadNotifications()}>
            清理已读
          </Button>
        </Space>
      </div>
      <Spin spinning={notificationLoading}>
        <div className="max-h-[360px] overflow-y-auto py-2">
          {notificationItems.length === 0 ? (
            <Empty className="my-8" image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无通知" />
          ) : (
            <div className="flex flex-col">
              {notificationItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  aria-label={`通知：${item.title}`}
                  className="grid w-full grid-cols-[8px_1fr] gap-3 border-0 border-b border-solid border-[#edf1f7] bg-white px-1 py-3 text-left transition last:border-b-0 hover:bg-[#f8fbff]"
                  onClick={() => void handleNotificationClick(item)}
                >
                  <span
                    aria-hidden="true"
                    className={[
                      "mt-[7px] h-2 w-2 rounded-full",
                      item.is_read ? "bg-[#cbd5e1]" : notificationLevelClassName(item.level),
                    ].join(" ")}
                  />
                  <span className="min-w-0">
                    <span className="flex items-center justify-between gap-3">
                      <span className="truncate text-[13px] font-semibold text-[#111827]">{item.title}</span>
                      <span className="shrink-0 text-[12px] text-[#8a94a6]">{formatNotificationTime(item.created_at)}</span>
                    </span>
                    <span className="mt-1 line-clamp-2 text-[12px] leading-5 text-[#64748b]">{item.content}</span>
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>
      </Spin>
    </div>
  );

  return (
    <Header className="app-glass-topbar sticky top-0 z-10 flex h-[72px] items-center gap-4 overflow-visible border-b border-[#e3e9f2] !px-8 shadow-none">
      <Tooltip title={locked ? lockedActionMessage : searchError ?? "按 Enter 搜索股票"}>
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
          disabled={locked || searching}
        />
      </Tooltip>
      <Tag className="m-0 flex h-9 shrink-0 items-center rounded-md border-[#d8e7ff] bg-[#edf5ff] px-4 text-[14px] font-medium leading-9 text-[#1677ff]">{quoteStatus.marketLabel}</Tag>
      <div className="flex min-w-[190px] shrink-0 items-center gap-3 whitespace-nowrap text-[14px] text-slate-500">
        <span>{locked ? "更新于" : "数据更新："}</span>
        <span>{quoteStatus.timeLabel}</span>
      </div>
      <Space className="ml-auto shrink-0" size={12} separator={<span className="h-5 w-px bg-[#e3e9f2]" />}>
        <Button className="h-9 px-4 text-[14px]" icon={<ReloadOutlined />} onClick={locked ? showLockedMessage : () => void load()}>
          刷新
        </Button>
        {locked ? (
          <Button aria-label="通知" className="h-9 px-3 text-[14px] text-[#9ca3af]" type="text" icon={<BellOutlined />} onClick={showLockedMessage}>
            通知
          </Button>
        ) : (
          <Popover
            arrow={false}
            content={notificationContent}
            open={notificationOpen}
            placement="bottomRight"
            trigger="click"
            onOpenChange={handleNotificationOpenChange}
          >
            <Badge count={notificationUnreadCount} size="small" overflowCount={99}>
              <Button aria-label="通知" className="h-9 px-3 text-[14px]" type="text" icon={<BellOutlined />}>
                通知
              </Button>
            </Badge>
          </Popover>
        )}
        <Button className="h-9 px-3 text-[14px]" type="text" icon={<SettingOutlined />} onClick={locked ? showLockedMessage : () => navigate("/settings")}>
          设置
        </Button>
      </Space>
    </Header>
  );
}

function resolveNotificationRoute(route?: string): string | null {
  const value = route?.trim() ?? "";
  if (!value) {
    return null;
  }
  const exactRoutes = new Set(["/", "/watchlist", "/analysis", "/reports", "/news", "/tasks", "/settings"]);
  if (exactRoutes.has(value)) {
    return value;
  }
  if (/^\/reports\/\d+$/.test(value)) {
    return value;
  }
  if (/^\/stocks\/[A-Za-z0-9._:%-]+$/.test(value)) {
    return value;
  }
  return null;
}

async function loadDesktopNotificationSettings(): Promise<DesktopNotificationSettings> {
  const result = await settingsGet([...desktopNotificationSettingKeys]);
  const values = new Map(result.items.map((item) => [item.key, item.value]));
  return {
    systemEnabled: settingBoolean(values, "notifications.system_enabled", true),
    taskSuccessNotification: settingBoolean(values, "notifications.task_success", true),
    taskFailedNotification: settingBoolean(values, "notifications.task_failed", true),
    providerErrorNotification: settingBoolean(values, "notifications.provider_error", true),
  };
}

function settingBoolean(values: Map<string, string>, key: string, fallback: boolean): boolean {
  const value = values.get(key);
  if (value === "true") {
    return true;
  }
  if (value === "false") {
    return false;
  }
  return fallback;
}

function notificationKindFromItem(item: NotificationItem): DesktopNotificationKind | null {
  if (item.type === "task_success" || item.type === "task_failed" || item.type === "provider_error") {
    return item.type;
  }
  return null;
}

function notificationLevelClassName(level: string): string {
  if (level === "success") {
    return "bg-[#16a34a]";
  }
  if (level === "warning") {
    return "bg-[#f97316]";
  }
  if (level === "error") {
    return "bg-[#ff4d4f]";
  }
  return "bg-[#1677ff]";
}

function formatNotificationTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  });
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

function ShellStatus(props: { locked?: boolean }) {
  const location = useLocation();
  const dashboardState = useDashboardStore((store) => store.state);
  const dashboardError = useDashboardStore((store) => store.error);
  const [status, setStatus] = useState<ShellRuntimeStatus>({
    health: null,
    providers: [],
    error: null,
  });

  useEffect(() => {
    if (props.locked) {
      return;
    }
    let active = true;
    const loadStatus = async () => {
      try {
        const healthRequest = coreHealth();
        const providersRequest = providersStatus()
          .then((result) => ({ providers: Array.isArray(result.items) ? result.items : [], error: null }))
          .catch((cause) => ({
            providers: [] as ProviderStatusItem[],
            error: cause instanceof Error ? cause.message : "数据源状态读取失败",
          }));
        const health = await healthRequest;
        const providerStatus = await providersRequest;
        if (!active) {
          return;
        }
        setStatus({ health, providers: providerStatus.providers, error: providerStatus.error });
      } catch (cause) {
        if (!active) {
          return;
        }
        setStatus({
          health: null,
          providers: [],
          error: cause instanceof Error ? cause.message : "本地核心服务连接失败",
        });
      }
    };

    void loadStatus();
    const interval = window.setInterval(() => void loadStatus(), 30_000);
    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, [location.pathname, props.locked]);

  if (props.locked) {
    return (
      <div className="rounded-lg border border-[#e3e9f2] px-5 py-4 text-[14px] text-slate-700 shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
        <div className="flex items-center gap-3">
          <Spin size="small" />
          <div>
            <div className="font-medium text-[#374151]">系统初始化中...</div>
            <div className="mt-1 text-[12px] text-[#64748b]">请稍候</div>
          </div>
        </div>
      </div>
    );
  }

  const health = status.health ?? dashboardState?.health ?? null;
  const providers = status.health ? status.providers ?? [] : dashboardState?.summary.provider_statuses ?? [];
  const statusError = status.error ?? dashboardError;
  const coreOK = Boolean(health);
  const sqliteOK = health?.dbStatus === "ok";
  const providerOK = providers.length > 0 && providers.every((provider) => provider.available);
  const providerError = providers.find((provider) => !provider.available)?.last_error || statusError;
  const rows = [
    { label: "Go Core", value: coreOK ? "已连接" : "未连接", ok: coreOK },
    { label: "SQLite", value: sqliteStatusText(health?.dbStatus, coreOK), ok: sqliteOK },
    { label: "数据源", value: coreOK ? (providerOK ? "正常" : "不可用") : "未知", ok: coreOK && providerOK },
    { label: "版本号", value: health?.version || "未知", ok: Boolean(health?.version) },
  ];
  return (
    <div className="app-glass-status-card rounded-lg border border-[#e3e9f2] px-5 py-4 text-[14px] text-slate-700 shadow-[0_4px_16px_rgba(15,23,42,0.04)]">
      {health ? (
        <span className="sr-only">
          本地核心服务已连接，版本 {health.version}
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
        <span>{health?.version || "未知"}</span>
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

type ShellRuntimeStatus = {
  health: CoreHealth | null;
  providers: ProviderStatusItem[];
  error: string | null;
};

function providerErrorText(value: string) {
  if (value.includes("market_provider_unconfigured")) {
    return "行情数据源未配置";
  }
  return value;
}

function sqliteStatusText(value: string | null | undefined, coreOK: boolean) {
  if (!coreOK) {
    return "未知";
  }
  if (value === "ok") {
    return "正常";
  }
  if (value === "not_configured") {
    return "未配置";
  }
  return value || "异常";
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
