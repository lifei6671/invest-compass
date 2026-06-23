import { Alert, App as AntApp, Button, ConfigProvider, Spin, Tag, Typography } from "antd";
import { Component, lazy, Suspense, useEffect, useMemo, useState, type ErrorInfo, type FormEvent, type ReactNode } from "react";
import { HashRouter, Link, Route, Routes } from "react-router-dom";
import { KlineChart } from "../components/market/KlineChart";
import { SchedulerBackfillDialog } from "../components/scheduler/SchedulerBackfillDialog";
import { SchedulerJobEditor, type SchedulerJobFormState } from "../components/scheduler/SchedulerJobEditor";
import { SchedulerJobTable } from "../components/scheduler/SchedulerJobTable";
import { SchedulerRunList } from "../components/scheduler/SchedulerRunList";
import {
  aiConfigDelete,
  aiConfigList,
  aiConfigSave,
  aiConfigTest,
  appBootStatus,
  autostartGet,
  autostartSet,
  cacheClean,
  cacheStats,
  checkUpdate,
  exportLogs,
  marketIndicators,
  marketKline,
  marketQuote,
  newsList,
  openExternalURL,
  promptTemplatesCreate,
  promptTemplatesDelete,
  promptTemplatesList,
  promptTemplatesUpdate,
  providersStatus,
  selectDirectory,
  settingsGet,
  settingsSet,
  stockSearch,
  watchlistCreate,
  watchlistDelete,
  watchlistList,
  watchlistUpdate,
  workspaceGet,
  workspaceSet,
  type AppBootStatus,
  type AIConfig,
  type AIConfigSavePayload,
  type AIConfigTestResult,
  type AutostartState,
  type CacheStatsResult,
  type ExportLogsResult,
  type MarketIndicatorsResult,
  type MarketKlineItem,
  type MarketQuote,
  type NewsItem,
  type PromptTemplate,
  type PromptTemplateCreatePayload,
  type PromptTemplateType,
  type ProviderStatusItem,
  type SettingItem,
  type StockSearchResult,
  type UpdateCheckResult,
  type WatchlistItem,
  type WorkspaceResult,
} from "../services/coreClient";
import {
  schedulerJobsBackfill,
  schedulerJobsDelete,
  schedulerJobsList,
  schedulerJobsRunNow,
  schedulerJobsSave,
  schedulerJobsSetEnabled,
  schedulerJobTypes,
  schedulerRefreshSymbol,
  schedulerRunsGet,
  schedulerRunsList,
  schedulerStatus,
  type SchedulerJob,
  type SchedulerJobPayload,
  type SchedulerJobType,
  type SchedulerRun,
  type SchedulerStatus,
} from "../services/scheduler";
import { APP_FONT } from "../styles/fonts";
import { appAntdLocale } from "../lib/antdLocale";
import { AppShell } from "./AppShell";
import { AppInitializationPage } from "../pages/initialization/AppInitializationPage";

const DashboardPage = lazy(() =>
  import("../pages/dashboard/DashboardPage").then((module) => ({
    default: module.DashboardPage,
  })),
);
const WatchlistPage = lazy(() =>
  import("../components/watchlist/WatchlistPage").then((module) => ({
    default: module.WatchlistPage,
  })),
);
const StockDetailPage = lazy(() =>
  import("../components/stock-detail/StockDetailPage").then((module) => ({
    default: module.StockDetailPage,
  })),
);
const AnalysisPage = lazy(() =>
  import("../pages/analysis/AnalysisPage").then((module) => ({
    default: module.AnalysisPage,
  })),
);
const AnalysisRunningPage = lazy(() =>
  import("../pages/analysis-running/AnalysisRunningPage").then((module) => ({
    default: module.AnalysisRunningPage,
  })),
);
const ReportHistoryPage = lazy(() =>
  import("../pages/reports/ReportHistoryPage").then((module) => ({
    default: module.ReportHistoryPage,
  })),
);
const ReportDetailPage = lazy(() =>
  import("../pages/reports/detail/ReportDetailPage").then((module) => ({
    default: module.ReportDetailPage,
  })),
);
const TaskHistoryPage = lazy(() =>
  import("../pages/tasks/TaskHistoryPage").then((module) => ({
    default: module.TaskHistoryPage,
  })),
);
const NewsCenterPage = lazy(() =>
  import("../pages/news/NewsCenterPage").then((module) => ({
    default: module.NewsCenterPage,
  })),
);
const SettingsPage = lazy(() =>
  import("../pages/settings/SettingsPage").then((module) => ({
    default: module.SettingsPage,
  })),
);

type AppErrorBoundaryProps = {
  children: ReactNode;
};

type AppErrorBoundaryState = {
  hasError: boolean;
  message: string;
  componentStack: string;
};

export class AppErrorBoundary extends Component<AppErrorBoundaryProps, AppErrorBoundaryState> {
  state: AppErrorBoundaryState = { hasError: false, message: "", componentStack: "" };

  static getDerivedStateFromError(): AppErrorBoundaryState {
    return { hasError: true, message: "", componentStack: "" };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // 渲染异常只进入本地边界，不把错误对象或可能包含的上下文透出到页面。
    this.setState({
      message: error.message,
      componentStack: info.componentStack ?? "",
    });
  }

  render() {
    if (this.state.hasError) {
      const isDevMode = Boolean(
        (import.meta as unknown as { env?: { DEV?: boolean } }).env?.DEV,
      );
      if (isDevMode && this.state.message) {
        return (
          <Alert
            title="界面渲染失败"
            description={
              <pre className="whitespace-pre-wrap break-words text-xs">
                {this.state.message}
                {this.state.componentStack ? `\n${this.state.componentStack}` : ""}
              </pre>
            }
            type="error"
            showIcon
          />
        );
      }
      return <Alert title="界面渲染失败" type="error" showIcon />;
    }

    return this.props.children;
  }
}

const APP_ROUTES = {
  home: "/",
  watchlist: "/watchlist",
  stockDetail: "/stocks/:symbol",
  news: "/news",
  scheduler: "/scheduler",
  analysis: "/analysis",
  analysisRunning: "/analysis/running",
  reports: "/reports",
  reportDetail: "/reports/:reportId",
  tasks: "/tasks",
  settings: "/settings",
  aiSettings: "/ai-settings",
} as const;

export const APP_NAV_ITEMS = [
  { path: APP_ROUTES.home, label: "总览" },
  { path: APP_ROUTES.watchlist, label: "自选股" },
  { path: APP_ROUTES.analysis, label: "AI 分析" },
  { path: APP_ROUTES.reports, label: "报告历史" },
  { path: APP_ROUTES.news, label: "资讯中心" },
  { path: APP_ROUTES.tasks, label: "任务历史" },
  { path: APP_ROUTES.settings, label: "设置" },
] as const;

export const APP_ROUTE_PATHS = Object.values(APP_ROUTES);

export type AppBootState = "initializing" | "ready" | "failed";

type AppProps = {
  initialBootState?: AppBootState;
  bootStatusPollIntervalMs?: number;
  minimumInitializationVisibleMs?: number;
};

const defaultBootStatusPollIntervalMs = 300;
const defaultMinimumInitializationVisibleMs = 3000;
const minimumReadyVisibleMs = 800;

export function App(props: AppProps = {}) {
  const [bootState, setBootState] = useState<AppBootState>(props.initialBootState ?? defaultBootState());
  const [bootStatus, setBootStatus] = useState<AppBootStatus | null>(null);
  const bootStatusPollIntervalMs = props.bootStatusPollIntervalMs ?? defaultBootStatusPollIntervalMs;
  const minimumInitializationVisibleMs = props.minimumInitializationVisibleMs ?? defaultMinimumInitializationVisibleMs;

  useEffect(() => {
    if (bootState !== "initializing") {
      return;
    }
    let active = true;
    let timer: number | undefined;
    const startedAt = Date.now();
    const pollBootStatus = async () => {
      try {
        const nextStatus = await appBootStatus();
        if (!active) {
          return;
        }
        setBootStatus(nextStatus);
        if (nextStatus.ready) {
          const remainingVisibleMs = Math.max(minimumReadyVisibleMs, minimumInitializationVisibleMs - (Date.now() - startedAt));
          timer = window.setTimeout(() => {
            if (active) {
              setBootState("ready");
            }
          }, remainingVisibleMs);
          return;
        }
      } catch {
        if (!active) {
          return;
        }
        // 启动状态 command 自身失败时，停留在初始化页并展示脱敏错误；下一轮轮询仍可恢复。
        setBootStatus(buildBootStatusReadFailedState());
      }
      if (active) {
        timer = window.setTimeout(pollBootStatus, bootStatusPollIntervalMs);
      }
    };
    void pollBootStatus();
    return () => {
      active = false;
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }, [bootState, bootStatusPollIntervalMs, minimumInitializationVisibleMs]);

  return (
    <ConfigProvider
      locale={appAntdLocale}
      theme={{
        token: {
          borderRadius: 4,
          colorPrimary: "#1677ff",
          colorTextSecondary: "#6b7280",
          colorBorder: "#e5e7eb",
          fontFamily: APP_FONT,
          fontSize: 13,
          boxShadowSecondary: "0 8px 24px rgba(15, 23, 42, 0.06)",
        },
      }}
    >
      <AntApp>
        <AppErrorBoundary>
          <HashRouter>
            {bootState === "initializing" ? (
              <AppInitializationPage state={bootStatus ?? undefined} />
            ) : (
              <AppShell routes={APP_ROUTES} navItems={APP_NAV_ITEMS}>
                <Suspense
                  fallback={
                    <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-sm text-blue-700">
                      正在连接本地核心服务 <Spin className="ml-2" size="small" />
                    </div>
                  }
                >
                  <Routes>
                    <Route path={APP_ROUTES.home} element={<DashboardPage />} />
                    <Route path={APP_ROUTES.watchlist} element={<WatchlistPage />} />
                    <Route path={APP_ROUTES.stockDetail} element={<StockDetailRoute />} />
                    <Route path={APP_ROUTES.news} element={<NewsCenterPage />} />
                    <Route path={APP_ROUTES.scheduler} element={<SchedulerRoute />} />
                    <Route path={APP_ROUTES.analysis} element={<AnalysisPage />} />
                    <Route path={APP_ROUTES.analysisRunning} element={<AnalysisRunningPage />} />
                    <Route path={APP_ROUTES.reports} element={<ReportHistoryPage />} />
                    <Route path={APP_ROUTES.reportDetail} element={<ReportDetailPage />} />
                    <Route path={APP_ROUTES.tasks} element={<TaskHistoryPage />} />
                    <Route path={APP_ROUTES.settings} element={<SettingsPage />} />
                    <Route path={APP_ROUTES.aiSettings} element={<AISettingsRoute />} />
                    <Route path="*" element={<Alert title="页面不存在" type="warning" showIcon />} />
                  </Routes>
                </Suspense>
              </AppShell>
            )}
          </HashRouter>
        </AppErrorBoundary>
      </AntApp>
    </ConfigProvider>
  );
}

function defaultBootState(): AppBootState {
  const env = (import.meta as unknown as { env?: { MODE?: string } }).env;
  return env?.MODE === "test" ? "ready" : "initializing";
}

function buildBootStatusReadFailedState(): AppBootStatus {
  return {
    ready: false,
    progress: 10,
    currentStepId: "sidecar",
    steps: [
      { id: "sidecar", index: 1, title: "启动 Go Core Sidecar", status: "failed", badgeText: "失败" },
      { id: "workspace", index: 2, title: "检查本地工作区与目录权限", status: "pending", badgeText: "等待中" },
      { id: "sqlite_migration", index: 3, title: "SQLite 数据库迁移 / 重建索引", status: "pending", badgeText: "等待中" },
      { id: "tokenizer", index: 4, title: "加载分词器（GSE / 全文检索词典）", status: "pending", badgeText: "等待中" },
      { id: "data_source", index: 5, title: "初始化数据源配置", status: "pending", badgeText: "等待中" },
      { id: "market_cache", index: 6, title: "同步基础行情快照与资讯缓存", status: "pending", badgeText: "等待中" },
      { id: "ready", index: 7, title: "完成基础检查并进入工作台", status: "pending", badgeText: "等待中" },
    ],
    taskDetail: {
      taskId: "boot-status-read",
      elapsed: "计算中",
      currentStage: "启动状态读取失败",
      remaining: "等待重试",
    },
    logs: [
      {
        id: "boot-status-read-failed",
        time: "--:--:--",
        status: "error",
        message: "启动状态读取失败，请稍后重试",
      },
    ],
  };
}

type SchedulerViewState = {
  status: SchedulerStatus;
  jobs: SchedulerJob[];
  runs: SchedulerRun[];
  types: SchedulerJobType[];
  providers: ProviderStatusItem[];
};

type AISettingsViewState = {
  configs: AIConfig[];
  templates: PromptTemplate[];
};

type SettingsViewState = {
  settings: SettingItem[];
  workspace: WorkspaceResult;
  cache: CacheStatsResult;
  providers: ProviderStatusItem[];
  autostart: AutostartState;
};

type SettingsFormState = {
  proxyURL: string;
  proxyPassword: string;
  proxyCredentialRef: string;
  closeToTray: boolean;
  autoStartEnabled: boolean;
  taskNotificationsEnabled: boolean;
  updateManifestURL: string;
  updateAllowedHosts: string;
  workspacePath: string;
};

type AIConfigFormState = {
  id: number;
  name: string;
  provider: string;
  baseUrl: string;
  apiKeyRef: string;
  maskedApiKey: string;
  hasApiKey: boolean;
  apiKey: string;
  modelName: string;
  temperature: string;
  maxTokens: string;
  timeoutSeconds: string;
  streamEnabled: boolean;
  isDefault: boolean;
};

type PromptTemplateFormState = {
  id: number;
  name: string;
  type: PromptTemplateType;
  description: string;
  content: string;
  isBuiltin: boolean;
};

type WatchlistViewState = {
  items: WatchlistItem[];
  quotes: Record<string, MarketQuote | { error: string }>;
};

type StockDetailViewState = {
  quote: MarketQuote;
  kline: MarketKlineItem[];
  indicators: MarketIndicatorsResult;
  news: NewsItem[];
};

function AISettingsRoute() {
  const [state, setState] = useState<AISettingsViewState | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [actionPending, setActionPending] = useState<string | null>(null);
  const [configForm, setConfigForm] = useState<AIConfigFormState>(() => defaultAIConfigForm());
  const [templateForm, setTemplateForm] = useState<PromptTemplateFormState>(() => defaultPromptTemplateForm());

  const load = () =>
    Promise.all([aiConfigList(), promptTemplatesList()]).then(([configs, templates]) => ({
      configs: configs.items,
      templates: templates.items,
    }));

  useEffect(() => {
    let active = true;
    load()
      .then((nextState) => {
        if (active) {
          setState(nextState);
          setLoadError(null);
        }
      })
      .catch((error: Error) => {
        if (active) {
          setLoadError(redactSensitiveText(error.message));
        }
      });
    return () => {
      active = false;
    };
  }, []);

  const saveConfig = (event: FormEvent) => {
    event.preventDefault();
    const validationError = validateAIConfigForm(configForm);
    if (validationError) {
      setActionError(validationError);
      return;
    }
    setActionError(null);
    setActionMessage(null);
    setActionPending("save-config");
    aiConfigSave(aiConfigPayloadFromForm(configForm))
      .then(({ config }) => {
        setState((current) => ({
          configs: upsertByID(current?.configs ?? [], config),
          templates: current?.templates ?? [],
        }));
        setConfigForm(configFormFromConfig(config));
        setActionMessage("模型配置已保存");
      })
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  const editConfig = (config: AIConfig) => {
    setActionError(null);
    setActionMessage(null);
    setConfigForm(configFormFromConfig(config));
  };

  const deleteConfig = (config: AIConfig) => {
    setActionError(null);
    setActionMessage(null);
    setActionPending(`delete-config-${config.id}`);
    aiConfigDelete({ id: config.id })
      .then(() => {
        setState((current) => ({
          configs: (current?.configs ?? []).filter((item) => item.id !== config.id),
          templates: current?.templates ?? [],
        }));
        if (configForm.id === config.id) {
          setConfigForm(defaultAIConfigForm());
        }
        setActionMessage(`${config.name || "模型配置"} 已删除`);
      })
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  const testConfig = (config: AIConfig) => {
    setActionError(null);
    setActionMessage(null);
    setActionPending(`test-config-${config.id}`);
    aiConfigTest({ id: config.id, api_key_ref: config.api_key_ref })
      .then((result: AIConfigTestResult) => {
        setActionMessage(result.ok ? `${config.name || "模型配置"} 连通性正常` : redactSensitiveText(result.message || "模型连通性测试失败"));
      })
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  const saveTemplate = (event: FormEvent) => {
    event.preventDefault();
    const validationError = validatePromptTemplateForm(templateForm);
    if (validationError) {
      setActionError(validationError);
      return;
    }
    setActionError(null);
    setActionMessage(null);
    setActionPending("save-template");
    const payload = promptTemplatePayloadFromForm(templateForm);
    const request = templateForm.id > 0 ? promptTemplatesUpdate({ ...payload, id: templateForm.id }) : promptTemplatesCreate(payload);
    request
      .then((template) => {
        setState((current) => ({
          configs: current?.configs ?? [],
          templates: upsertByID(current?.templates ?? [], template),
        }));
        setTemplateForm(promptTemplateFormFromTemplate(template));
        setActionMessage("Prompt 模板已保存");
      })
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  const editTemplate = (template: PromptTemplate) => {
    setActionError(null);
    setActionMessage(null);
    setTemplateForm(promptTemplateFormFromTemplate(template));
  };

  const deleteTemplate = (template: PromptTemplate) => {
    setActionError(null);
    setActionMessage(null);
    setActionPending(`delete-template-${template.id}`);
    promptTemplatesDelete(template.id)
      .then(() => {
        setState((current) => ({
          configs: current?.configs ?? [],
          templates: (current?.templates ?? []).filter((item) => item.id !== template.id),
        }));
        if (templateForm.id === template.id) {
          setTemplateForm(defaultPromptTemplateForm());
        }
        setActionMessage(`${template.name || "Prompt 模板"} 已删除`);
      })
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  if (loadError) {
    return <Alert title="模型配置读取失败" description={loadError} type="error" showIcon />;
  }
  if (!state) {
    return <Alert title="正在读取模型配置" description={<Spin size="small" />} type="info" showIcon />;
  }

  return (
    <section className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Typography.Title level={2} className="m-0">
          模型配置
        </Typography.Title>
        <Typography.Text type="secondary">OpenAI-compatible Provider、一次性 API Key 写入和 Prompt 模板管理。</Typography.Text>
      </div>
      {actionError ? <Alert title={actionError} type="error" showIcon /> : null}
      {actionMessage ? <Alert title={actionMessage} type="success" showIcon /> : null}
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            模型接入
          </Typography.Title>
          <Button htmlType="button" onClick={() => setConfigForm(defaultAIConfigForm())}>
            新建配置
          </Button>
        </div>
        <form className="grid gap-3 px-4 py-4 md:grid-cols-4" onSubmit={saveConfig}>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">配置名称</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              value={configForm.name}
              onChange={(event) => setConfigForm((current) => ({ ...current, name: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">Provider</span>
            <select
              className="h-8 rounded border border-slate-300 px-2"
              value={configForm.provider}
              onChange={(event) => setConfigForm((current) => ({ ...current, provider: event.target.value }))}
            >
              <option value="openai-compatible">OpenAI-compatible</option>
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="text-xs text-slate-500">接入点</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              value={configForm.baseUrl}
              onChange={(event) => setConfigForm((current) => ({ ...current, baseUrl: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">模型名称</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              value={configForm.modelName}
              onChange={(event) => setConfigForm((current) => ({ ...current, modelName: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">API Key</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              type="password"
              value={configForm.apiKey}
              onChange={(event) => setConfigForm((current) => ({ ...current, apiKey: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">Temperature</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              inputMode="decimal"
              value={configForm.temperature}
              onChange={(event) => setConfigForm((current) => ({ ...current, temperature: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">Max Tokens</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              inputMode="numeric"
              value={configForm.maxTokens}
              onChange={(event) => setConfigForm((current) => ({ ...current, maxTokens: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">超时秒数</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              inputMode="numeric"
              value={configForm.timeoutSeconds}
              onChange={(event) => setConfigForm((current) => ({ ...current, timeoutSeconds: event.target.value }))}
            />
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={configForm.streamEnabled}
              onChange={(event) => setConfigForm((current) => ({ ...current, streamEnabled: event.target.checked }))}
            />
            启用流式
          </label>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={configForm.isDefault}
              onChange={(event) => setConfigForm((current) => ({ ...current, isDefault: event.target.checked }))}
            />
            默认配置
          </label>
          <div className="flex flex-wrap gap-2 md:col-span-4">
            <Button htmlType="submit" loading={actionPending === "save-config"}>
              保存模型配置
            </Button>
            <Button htmlType="button" onClick={() => setConfigForm(defaultAIConfigForm())}>
              重置
            </Button>
          </div>
        </form>
        {state.configs.length === 0 ? (
          <div className="border-t border-slate-100 px-4 py-8 text-sm text-slate-500">暂无模型配置</div>
        ) : (
          <div className="overflow-x-auto border-t border-slate-100">
            <table className="w-full table-fixed text-left text-sm">
              <thead className="bg-slate-50 text-slate-500">
                <tr>
                  <th className="w-[22%] px-4 py-2 font-medium">名称</th>
                  <th className="w-[18%] px-4 py-2 font-medium">模型</th>
                  <th className="w-[28%] px-4 py-2 font-medium">接入点</th>
                  <th className="w-[14%] px-4 py-2 font-medium">密钥</th>
                  <th className="w-[18%] px-4 py-2 font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                {state.configs.map((config) => (
                  <tr key={config.id} className="border-t border-slate-100">
                    <td className="truncate px-4 py-3">
                      {config.name || `模型配置 ${config.id}`}
                      {config.is_default ? <Tag className="ml-2">默认</Tag> : null}
                    </td>
                    <td className="truncate px-4 py-3">{config.model_name || "-"}</td>
                    <td className="truncate px-4 py-3">{config.base_url || "-"}</td>
                    <td className="truncate px-4 py-3">{config.has_api_key ? config.masked_api_key || "已配置" : "未配置"}</td>
                    <td className="flex flex-wrap gap-2 px-4 py-3">
                      <Button size="small" loading={actionPending === `test-config-${config.id}`} onClick={() => testConfig(config)}>
                        测试 {config.name || `模型配置 ${config.id}`}
                      </Button>
                      <Button size="small" onClick={() => editConfig(config)}>
                        编辑
                      </Button>
                      <Button size="small" danger loading={actionPending === `delete-config-${config.id}`} onClick={() => deleteConfig(config)}>
                        删除
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="flex flex-col gap-2 border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            Prompt 模板
          </Typography.Title>
          <div className="flex flex-wrap gap-2 text-xs text-slate-500">
            {allowedPromptTemplateTypes.map((type) => (
              <Tag key={type}>{type}</Tag>
            ))}
            {allowedPromptVariables.map((variable) => (
              <Tag key={variable} color="blue">
                {`{{${variable}}}`}
              </Tag>
            ))}
          </div>
        </div>
        <form className="grid gap-3 px-4 py-4 md:grid-cols-4" onSubmit={saveTemplate}>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">模板名称</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              value={templateForm.name}
              onChange={(event) => setTemplateForm((current) => ({ ...current, name: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">模板类型</span>
            <select
              className="h-8 rounded border border-slate-300 px-2"
              value={templateForm.type}
              onChange={(event) => setTemplateForm((current) => ({ ...current, type: event.target.value as PromptTemplateType }))}
            >
              {allowedPromptTemplateTypes.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="text-xs text-slate-500">模板说明</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              value={templateForm.description}
              onChange={(event) => setTemplateForm((current) => ({ ...current, description: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm md:col-span-4">
            <span className="text-xs text-slate-500">模板内容</span>
            <textarea
              className="min-h-36 rounded border border-slate-300 px-2 py-2 font-mono text-xs"
              value={templateForm.content}
              onChange={(event) => setTemplateForm((current) => ({ ...current, content: event.target.value }))}
            />
          </label>
          <div className="flex flex-wrap gap-2 md:col-span-4">
            <Button htmlType="submit" loading={actionPending === "save-template"} disabled={templateForm.isBuiltin}>
              保存 Prompt 模板
            </Button>
            <Button htmlType="button" onClick={() => setTemplateForm(defaultPromptTemplateForm())}>
              重置
            </Button>
          </div>
        </form>
        {state.templates.length === 0 ? (
          <div className="border-t border-slate-100 px-4 py-8 text-sm text-slate-500">暂无 Prompt 模板</div>
        ) : (
          <div className="grid gap-0 border-t border-slate-100">
            {state.templates.map((template) => (
              <div key={template.id} className="grid gap-2 border-t border-slate-100 px-4 py-3 text-sm md:grid-cols-[1fr_auto_auto]">
                <div className="min-w-0">
                  <div className="truncate font-medium text-slate-900">
                    {template.name}
                    {template.is_builtin ? <Tag className="ml-2">内置</Tag> : null}
                  </div>
                  <div className="mt-1 line-clamp-2 text-xs text-slate-500">{template.description || template.content}</div>
                </div>
                <div className="flex flex-wrap items-start gap-1">
                  <Tag>{template.type}</Tag>
                  {template.variables.map((variable) => (
                    <Tag key={variable} color={allowedPromptVariableSet.has(variable) ? "blue" : "red"}>
                      {variable}
                    </Tag>
                  ))}
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button size="small" onClick={() => editTemplate(template)}>
                    编辑
                  </Button>
                  <Button
                    size="small"
                    danger
                    disabled={template.is_builtin}
                    loading={actionPending === `delete-template-${template.id}`}
                    onClick={() => deleteTemplate(template)}
                  >
                    删除
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </section>
  );
}

function SettingsRoute() {
  const [state, setState] = useState<SettingsViewState | null>(null);
  const [form, setForm] = useState<SettingsFormState>(() => defaultSettingsForm());
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [actionPending, setActionPending] = useState<string | null>(null);
  const [updateResult, setUpdateResult] = useState<UpdateCheckResult | null>(null);
  const [logTargetDir, setLogTargetDir] = useState("");
  const [logResult, setLogResult] = useState<ExportLogsResult | null>(null);

  const load = () =>
    Promise.all([settingsGet(settingsPageKeys), workspaceGet(), cacheStats(), providersStatus(), autostartGet()]).then(([settings, workspace, cache, providers, autostart]) => ({
      settings: settings.items,
      workspace,
      cache,
      providers: providers.items,
      autostart,
    }));

  useEffect(() => {
    let active = true;
    load()
      .then((nextState) => {
        if (!active) {
          return;
        }
        setState(nextState);
        setForm(settingsFormFromState(nextState));
        setLoadError(null);
      })
      .catch((error: Error) => {
        if (active) {
          setLoadError(redactSensitiveText(error.message));
        }
      });
    return () => {
      active = false;
    };
  }, []);

  const saveSettings = (event: FormEvent) => {
    event.preventDefault();
    const validationError = validateSettingsForm(form);
    if (validationError) {
      setActionError(validationError);
      return;
    }
    setActionError(null);
    setActionMessage(null);
    setActionPending("settings");
    settingsSet(settingsPayloadFromForm(form))
      .then(() => autostartSet(form.autoStartEnabled))
      .then(() => {
        setForm((current) => ({ ...current, proxyPassword: "" }));
        setActionMessage("设置已保存");
      })
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  const saveWorkspace = () => {
    setActionError(null);
    setActionMessage(null);
    setActionPending("workspace");
    workspaceSet(form.workspacePath)
      .then((workspace) => {
        setState((current) => (current ? { ...current, workspace } : current));
        setActionMessage("工作区已保存");
      })
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  const cleanCache = (target: string) => {
    setActionError(null);
    setActionMessage(null);
    setActionPending(`cache-${target}`);
    cacheClean([target])
      .then(() => cacheStats())
      .then((cache) => {
        setState((current) => (current ? { ...current, cache } : current));
        setActionMessage(`缓存 ${target} 已清理`);
      })
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  const checkForUpdate = () => {
    setActionError(null);
    setActionMessage(null);
    setActionPending("update");
    checkUpdate()
      .then((result) => setUpdateResult(result))
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  const exportLocalLogs = () => {
    const targetDir = logTargetDir.trim();
    setActionError(null);
    setActionMessage(null);
    setLogResult(null);
    if (!targetDir) {
      setActionError("日志导出目录不能为空");
      return;
    }
    setActionPending("logs");
    exportLogs(targetDir)
      .then((result) => {
        setLogResult(result);
        setActionMessage("日志已导出");
      })
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  const selectLogExportDirectory = () => {
    setActionError(null);
    setActionMessage(null);
    setActionPending("log-directory");
    selectDirectory()
      .then((selectedPath) => {
        if (selectedPath) {
          setLogTargetDir(selectedPath);
        }
      })
      .catch((error: Error) => setActionError(redactSensitiveText(error.message)))
      .finally(() => setActionPending(null));
  };

  if (loadError) {
    return <Alert title="设置中心读取失败" description={loadError} type="error" showIcon />;
  }
  if (!state) {
    return <Alert title="正在读取设置中心" description={<Spin size="small" />} type="info" showIcon />;
  }

  return (
    <section className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Typography.Title level={2} className="m-0">
          设置中心
        </Typography.Title>
        <Typography.Text type="secondary">工作区、代理、缓存、数据源、更新和本地日志。</Typography.Text>
      </div>
      {actionError ? <Alert title={actionError} type="error" showIcon /> : null}
      {actionMessage ? <Alert title={actionMessage} type="success" showIcon /> : null}
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            基础设置
          </Typography.Title>
        </div>
        <form className="grid gap-3 px-4 py-4 md:grid-cols-3" onSubmit={saveSettings}>
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="text-xs text-slate-500">代理 URL</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              placeholder="代理地址"
              value={form.proxyURL}
              onChange={(event) => setForm((current) => ({ ...current, proxyURL: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">代理密码</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              type="password"
              value={form.proxyPassword}
              onChange={(event) => setForm((current) => ({ ...current, proxyPassword: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm md:col-span-2">
            <span className="text-xs text-slate-500">更新 Manifest URL</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              value={form.updateManifestURL}
              onChange={(event) => setForm((current) => ({ ...current, updateManifestURL: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">更新允许域名</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              value={form.updateAllowedHosts}
              onChange={(event) => setForm((current) => ({ ...current, updateAllowedHosts: event.target.value }))}
            />
          </label>
          <label className="flex items-center gap-2 text-sm md:col-span-3">
            <input
              type="checkbox"
              checked={form.closeToTray}
              onChange={(event) => setForm((current) => ({ ...current, closeToTray: event.target.checked }))}
            />
            <span>关闭到托盘</span>
          </label>
          <label className="flex items-center gap-2 text-sm md:col-span-3">
            <input
              type="checkbox"
              checked={form.autoStartEnabled}
              onChange={(event) => setForm((current) => ({ ...current, autoStartEnabled: event.target.checked }))}
            />
            <span>开机自动启动</span>
          </label>
          <label className="flex items-center gap-2 text-sm md:col-span-3">
            <input
              type="checkbox"
              checked={form.taskNotificationsEnabled}
              onChange={(event) => setForm((current) => ({ ...current, taskNotificationsEnabled: event.target.checked }))}
            />
            <span>任务成功/失败通知</span>
          </label>
          <div className="flex flex-wrap gap-2 md:col-span-3">
            <Button htmlType="submit" loading={actionPending === "settings"}>
              保存设置
            </Button>
            <Tag>{form.proxyCredentialRef ? "代理凭据已保存" : "未保存代理凭据"}</Tag>
          </div>
        </form>
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            工作区
          </Typography.Title>
        </div>
        <div className="grid gap-3 px-4 py-4 md:grid-cols-[1fr_auto]">
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">工作区路径</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              value={form.workspacePath}
              onChange={(event) => setForm((current) => ({ ...current, workspacePath: event.target.value }))}
            />
          </label>
          <Button className="self-end" loading={actionPending === "workspace"} onClick={saveWorkspace}>
            保存工作区
          </Button>
        </div>
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            缓存
          </Typography.Title>
          <Typography.Text type="secondary">{formatBytes(state.cache.total_bytes)}</Typography.Text>
        </div>
        {state.cache.items.length === 0 ? (
          <div className="px-4 py-8 text-sm text-slate-500">暂无可清理缓存</div>
        ) : (
          <div className="grid gap-0">
            {state.cache.items.map((item) => (
              <div key={item.target} className="grid gap-3 border-t border-slate-100 px-4 py-3 text-sm md:grid-cols-[1fr_auto_auto]">
                <div>
                  <div className="font-medium text-slate-900">{item.label}</div>
                  <div className="mt-1 text-xs text-slate-500">{item.target}</div>
                </div>
                <span>{formatBytes(item.bytes)}</span>
                <Button size="small" disabled={!item.cleanable} loading={actionPending === `cache-${item.target}`} onClick={() => cleanCache(item.target)}>
                  清理 {item.target}
                </Button>
              </div>
            ))}
          </div>
        )}
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            数据刷新
          </Typography.Title>
          <Link to="/scheduler">进入任务调度</Link>
        </div>
        <div className="px-4 py-3 text-sm text-slate-600">管理交易日定时刷新、启动补偿、手动补偿和单股刷新。</div>
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            数据源
          </Typography.Title>
        </div>
        {state.providers.map((provider) => (
          <div key={`${provider.name}-${provider.source}`} className="grid gap-2 border-t border-slate-100 px-4 py-3 text-sm md:grid-cols-[1fr_auto_auto]">
            <span className="font-medium text-slate-900">{provider.name}</span>
            <span>{provider.source}</span>
            <Tag color={provider.available ? "green" : "red"}>{provider.available ? "可用" : "不可用"}</Tag>
            {provider.last_error ? <span className="text-xs text-red-600 md:col-span-3">{provider.last_error}</span> : null}
          </div>
        ))}
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            更新和日志
          </Typography.Title>
          <Button loading={actionPending === "update"} onClick={checkForUpdate}>
            检查更新
          </Button>
        </div>
        {updateResult ? (
          <div className="border-t border-slate-100 px-4 py-3 text-sm">
            <div>
              当前版本：<span>{updateResult.current_version}</span>
            </div>
            <div>
              最新版本：<span>{updateResult.latest_version}</span>
            </div>
            {updateResult.release_notes ? <div className="mt-1 text-slate-600">{updateResult.release_notes}</div> : null}
          </div>
        ) : null}
        <div className="grid gap-3 border-t border-slate-100 px-4 py-4 md:grid-cols-[1fr_auto_auto]">
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">日志导出目录</span>
            <input className="h-8 rounded border border-slate-300 px-2" value={logTargetDir} onChange={(event) => setLogTargetDir(event.target.value)} />
          </label>
          <Button className="self-end" loading={actionPending === "log-directory"} onClick={selectLogExportDirectory}>
            选择目录
          </Button>
          <Button className="self-end" loading={actionPending === "logs"} onClick={exportLocalLogs}>
            导出日志
          </Button>
          {logResult ? <div className="text-xs text-slate-500 md:col-span-3">{logResult.file_path}</div> : null}
        </div>
      </section>
      <section className="rounded-md border border-slate-200 bg-white px-4 py-4">
        <Typography.Title level={4} className="m-0">
          关于应用
        </Typography.Title>
        <div className="mt-3 flex flex-wrap gap-2 text-sm">
          <Tag>FREE</Tag>
          <span className="text-slate-600">仅作研究辅助，不构成投资建议。</span>
        </div>
      </section>
    </section>
  );
}

const allowedPromptTemplateTypes: PromptTemplateType[] = ["system", "stock_full", "technical", "custom"];
const allowedPromptVariables = ["stock_name", "stock_code", "market", "quote", "kline_summary", "indicators", "news", "analysis_language"];
const allowedPromptVariableSet = new Set(allowedPromptVariables);
const settingsPageKeys = ["proxy_url", "proxy_credential_ref", "window.close_to_tray", "notifications.task_terminal", "update.manifest_url", "update.allowed_hosts"];

function defaultSettingsForm(): SettingsFormState {
  return {
    proxyURL: "",
    proxyPassword: "",
    proxyCredentialRef: "",
    closeToTray: false,
    autoStartEnabled: false,
    taskNotificationsEnabled: true,
    updateManifestURL: "",
    updateAllowedHosts: "",
    workspacePath: "",
  };
}

function settingsFormFromState(state: SettingsViewState): SettingsFormState {
  return {
    proxyURL: settingValue(state.settings, "proxy_url"),
    proxyPassword: "",
    proxyCredentialRef: settingValue(state.settings, "proxy_credential_ref"),
    closeToTray: settingValue(state.settings, "window.close_to_tray").trim().toLowerCase() === "true",
    autoStartEnabled: state.autostart.enabled,
    taskNotificationsEnabled: taskNotificationsEnabledFromSettings(state.settings),
    updateManifestURL: settingValue(state.settings, "update.manifest_url"),
    updateAllowedHosts: settingValue(state.settings, "update.allowed_hosts"),
    workspacePath: state.workspace.path,
  };
}

function settingsPayloadFromForm(form: SettingsFormState) {
  return {
    items: [
      { key: "proxy_url", value: form.proxyURL.trim() },
      { key: "window.close_to_tray", value: String(form.closeToTray) },
      { key: "notifications.task_terminal", value: String(form.taskNotificationsEnabled) },
      { key: "update.manifest_url", value: form.updateManifestURL.trim() },
      { key: "update.allowed_hosts", value: form.updateAllowedHosts.trim() },
    ],
    proxy_password: form.proxyPassword.trim() || undefined,
    proxy_credential_ref: form.proxyCredentialRef,
    clear_proxy_credential: false,
  };
}

function settingValue(items: SettingItem[], key: string) {
  return items.find((item) => item.key === key)?.value ?? "";
}

function taskNotificationsEnabledFromSettings(items: SettingItem[]) {
  return settingValue(items, "notifications.task_terminal").trim().toLowerCase() !== "false";
}

function validateSettingsForm(form: SettingsFormState) {
  const proxyURL = form.proxyURL.trim();
  if (!proxyURL) {
    return "";
  }
  try {
    const parsed = new URL(proxyURL);
    if (parsed.username || parsed.password) {
      return "代理 URL 不能包含用户名或密码";
    }
  } catch {
    return "代理 URL 格式不正确";
  }
  return "";
}

function formatBytes(value: number) {
  if (value < 1024) {
    return `${value} B`;
  }
  return `${(value / 1024).toFixed(1)} KiB`;
}

function defaultAIConfigForm(): AIConfigFormState {
  return {
    id: 0,
    name: "",
    provider: "openai-compatible",
    baseUrl: "",
    apiKeyRef: "",
    maskedApiKey: "",
    hasApiKey: false,
    apiKey: "",
    modelName: "gpt-4.1-mini",
    temperature: "0.2",
    maxTokens: "4096",
    timeoutSeconds: "120",
    streamEnabled: true,
    isDefault: true,
  };
}

function configFormFromConfig(config: AIConfig): AIConfigFormState {
  return {
    id: config.id,
    name: config.name || "",
    provider: config.provider || "openai-compatible",
    baseUrl: config.base_url || "",
    apiKeyRef: config.api_key_ref || "",
    maskedApiKey: config.masked_api_key || "",
    hasApiKey: Boolean(config.has_api_key),
    apiKey: "",
    modelName: config.model_name || "",
    temperature: String(config.temperature ?? 0.2),
    maxTokens: String(config.max_tokens ?? 4096),
    timeoutSeconds: String(config.timeout_seconds ?? 120),
    streamEnabled: Boolean(config.stream_enabled),
    isDefault: Boolean(config.is_default),
  };
}

function aiConfigPayloadFromForm(form: AIConfigFormState): AIConfigSavePayload {
  const apiKey = form.apiKey.trim();
  return {
    id: form.id,
    name: form.name.trim(),
    provider: form.provider.trim(),
    base_url: form.baseUrl.trim(),
    api_key_ref: form.apiKeyRef,
    masked_api_key: form.maskedApiKey,
    has_api_key: form.hasApiKey || apiKey.length > 0,
    model_name: form.modelName.trim(),
    temperature: Number(form.temperature),
    max_tokens: Number(form.maxTokens),
    timeout_seconds: Number(form.timeoutSeconds),
    stream_enabled: form.streamEnabled,
    is_default: form.isDefault,
    api_key: apiKey || undefined,
  };
}

function validateAIConfigForm(form: AIConfigFormState) {
  if (!form.name.trim()) {
    return "请填写配置名称";
  }
  if (!form.baseUrl.trim()) {
    return "请填写接入点";
  }
  if (!form.modelName.trim()) {
    return "请填写模型名称";
  }
  if (!Number.isFinite(Number(form.temperature))) {
    return "Temperature 必须是数字";
  }
  if (!positiveInteger(form.maxTokens)) {
    return "Max Tokens 必须是正整数";
  }
  if (!positiveInteger(form.timeoutSeconds)) {
    return "超时秒数必须是正整数";
  }
  return "";
}

function defaultPromptTemplateForm(): PromptTemplateFormState {
  return {
    id: 0,
    name: "",
    type: "stock_full",
    description: "",
    content: "",
    isBuiltin: false,
  };
}

function promptTemplateFormFromTemplate(template: PromptTemplate): PromptTemplateFormState {
  return {
    id: template.id,
    name: template.name || "",
    type: template.type,
    description: template.description || "",
    content: template.content || "",
    isBuiltin: Boolean(template.is_builtin),
  };
}

function promptTemplatePayloadFromForm(form: PromptTemplateFormState): PromptTemplateCreatePayload {
  return {
    name: form.name.trim(),
    type: form.type,
    description: form.description.trim(),
    content: form.content,
  };
}

function validatePromptTemplateForm(form: PromptTemplateFormState) {
  if (form.isBuiltin) {
    return "内置 Prompt 模板不可直接修改";
  }
  if (!form.name.trim()) {
    return "请填写模板名称";
  }
  if (!allowedPromptTemplateTypes.includes(form.type)) {
    return `模板类型 ${form.type} 不在首版白名单`;
  }
  if (!form.content.trim()) {
    return "请填写模板内容";
  }
  for (const variable of extractPromptVariables(form.content)) {
    if (!allowedPromptVariableSet.has(variable)) {
      return `变量 ${variable} 不在首版白名单`;
    }
  }
  return "";
}

function extractPromptVariables(content: string) {
  const variables: string[] = [];
  const pattern = /\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}/g;
  let match = pattern.exec(content);
  while (match) {
    variables.push(match[1]);
    match = pattern.exec(content);
  }
  return uniqueValues(variables);
}

function redactSensitiveText(value: string) {
  return value
    .replace(/\bsk-[A-Za-z0-9_-]+\b/g, "[已脱敏]")
    .replace(/\bBearer\s+[A-Za-z0-9._~+/=-]+\b/gi, "Bearer [已脱敏]")
    .replace(/\bAuthorization:\s*[^\s]+/gi, "Authorization: [已脱敏]");
}

function upsertByID<T extends { id: number }>(items: T[], item: T) {
  const exists = items.some((current) => current.id === item.id);
  if (!exists) {
    return [...items, item];
  }
  return items.map((current) => (current.id === item.id ? item : current));
}

function positiveInteger(value: string) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0;
}

function WatchlistRoute() {
  const [state, setState] = useState<WatchlistViewState | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [searchKeyword, setSearchKeyword] = useState("");
  const [searchResults, setSearchResults] = useState<StockSearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [refreshingSymbols, setRefreshingSymbols] = useState<Record<string, boolean>>({});

  const load = () =>
    watchlistList().then(async (list) => ({
      items: list.items,
      quotes: await loadQuotes(list.items),
    }));

  useEffect(() => {
    let active = true;
    load()
      .then((nextState) => {
        if (active) {
          setState(nextState);
          setLoadError(null);
        }
      })
      .catch((error: Error) => {
        if (active) {
          setLoadError(error.message);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  const refresh = () => {
    setActionError(null);
    load()
      .then((nextState) => {
        setState(nextState);
        setActionMessage("自选股已刷新");
      })
      .catch((error: Error) => setActionError(error.message));
  };

  const search = (event: FormEvent) => {
    event.preventDefault();
    setActionError(null);
    setSearching(true);
    stockSearch(searchKeyword)
      .then((results) => setSearchResults(results))
      .catch((error: Error) => setActionError(error.message))
      .finally(() => setSearching(false));
  };

  const add = (symbol: string) => {
    setActionError(null);
    const sortOrder = state ? state.items.length * 10 + 20 : 10;
    watchlistCreate({ symbol, sort_order: sortOrder, tags: [], note: "" })
      .then(async (item) => {
        const quote = await loadQuote(item.symbol);
        setState((current) => ({
          items: [...(current?.items ?? []), item],
          quotes: { ...(current?.quotes ?? {}), [item.symbol]: quote },
        }));
        setActionMessage(`${item.symbol} 已添加`);
      })
      .catch((error: Error) => setActionError(error.message));
  };

  const updateDraft = (id: number, patch: Partial<WatchlistItem>) => {
    setState((current) =>
      current
        ? {
          ...current,
          items: current.items.map((item) => (item.id === id ? { ...item, ...patch } : item)),
        }
        : current,
    );
  };

  const save = (item: WatchlistItem) => {
    setActionError(null);
    watchlistUpdate({
      id: item.id,
      sort_order: Number(item.sort_order) || 0,
      tags: item.tags,
      note: item.note,
    })
      .then((updated) => {
        updateDraft(updated.id, updated);
        setActionMessage(`${updated.symbol} 已保存`);
      })
      .catch((error: Error) => setActionError(error.message));
  };

  const remove = (item: WatchlistItem) => {
    setActionError(null);
    watchlistDelete(item.id)
      .then(() => {
        setState((current) =>
          current
            ? {
              items: current.items.filter((value) => value.id !== item.id),
              quotes: Object.fromEntries(Object.entries(current.quotes).filter(([symbol]) => symbol !== item.symbol)),
            }
            : current,
        );
        setActionMessage(`${item.symbol} 已删除`);
      })
      .catch((error: Error) => setActionError(error.message));
  };

  const refreshSymbol = (item: WatchlistItem) => {
    setActionError(null);
    setActionMessage(null);
    setRefreshingSymbols((current) => ({ ...current, [item.symbol]: true }));
    schedulerRefreshSymbol({ symbol: item.symbol, data_type: "quote" })
      .then(async (result) => {
        const pendingRun = result.items.find((run) => run.status === "queued" || run.status === "running");
        if (pendingRun) {
          setActionMessage(`${item.symbol} 行情刷新任务已提交`);
          return;
        }
        const failedRun = result.items.find((run) => run.status === "failed" || run.status === "cancelled");
        if (failedRun) {
          throw new Error(failedRun.error_message || `${item.symbol} 行情刷新失败`);
        }
        const skippedRun = result.items.find((run) => run.status === "skipped");
        if (skippedRun) {
          setActionMessage(`${item.symbol} 行情刷新已跳过：${skippedRun.skipped_reason || "重复任务"}`);
          return;
        }
        if (result.items.length === 0 || !result.items.every((run) => run.status === "success")) {
          setActionMessage(`${item.symbol} 行情刷新任务已提交`);
          return;
        }
        const quote = await loadQuote(item.symbol);
        setState((current) =>
          current
            ? {
              ...current,
              quotes: { ...current.quotes, [item.symbol]: quote },
            }
            : current,
        );
        setActionMessage(`${item.symbol} 行情已刷新`);
      })
      .catch((error: Error) => setActionError(error.message))
      .finally(() =>
        setRefreshingSymbols((current) => {
          const next = { ...current };
          delete next[item.symbol];
          return next;
        }),
      );
  };

  if (loadError) {
    return <Alert title="自选股读取失败" description={loadError} type="error" showIcon />;
  }
  if (!state) {
    return <Alert title="正在读取自选股" description={<Spin size="small" />} type="info" showIcon />;
  }

  const sortedItems = [...state.items].sort((left, right) => left.sort_order - right.sort_order || left.symbol.localeCompare(right.symbol));

  return (
    <section className="flex flex-col gap-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <Typography.Title level={2} className="m-0">
            自选股
          </Typography.Title>
          <Typography.Text type="secondary">搜索、维护自选股，并读取真实行情快照。</Typography.Text>
        </div>
        <Button aria-label="刷新" onClick={refresh}>刷新</Button>
      </div>

      {actionError ? <Alert title={actionError} type="error" showIcon /> : null}
      {actionMessage ? <Alert title={actionMessage} type="success" showIcon /> : null}

      <form onSubmit={search} className="flex flex-wrap items-end gap-3 rounded-md border border-slate-200 bg-white p-4">
        <label className="flex min-w-60 flex-1 flex-col gap-1 text-sm">
          <span className="text-slate-600">搜索股票</span>
          <input
            aria-label="搜索股票"
            className="rounded border border-slate-300 px-3 py-2"
            value={searchKeyword}
            onChange={(event) => setSearchKeyword(event.target.value)}
          />
        </label>
        <Button aria-label="搜索" htmlType="submit" loading={searching}>搜索</Button>
      </form>

      {searchResults.length > 0 ? (
        <section className="rounded-md border border-slate-200 bg-white">
          <div className="border-b border-slate-200 px-4 py-3 text-sm font-medium text-slate-700">搜索结果</div>
          {searchResults.map((result) => (
            <div key={result.symbol} className="flex items-center justify-between border-t border-slate-100 px-4 py-3 text-sm">
              <div>
                <div className="font-medium text-slate-900">{result.name}</div>
                <div className="mt-1 text-xs text-slate-500">{[result.code, result.market, result.exchange].filter(Boolean).join(" · ")}</div>
              </div>
              <Button aria-label={`添加 ${result.symbol}`} onClick={() => add(result.symbol)}>添加 {result.symbol}</Button>
            </div>
          ))}
        </section>
      ) : null}

      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3 text-sm font-medium text-slate-700">自选列表</div>
        {sortedItems.length === 0 ? (
          <div className="px-4 py-8 text-sm text-slate-500">暂无自选股</div>
        ) : (
          sortedItems.map((item) => (
            <WatchlistRow
              key={item.id}
              item={item}
              quote={state.quotes[item.symbol]}
              refreshing={Boolean(refreshingSymbols[item.symbol])}
              onDraftChange={updateDraft}
              onRefresh={refreshSymbol}
              onSave={save}
              onDelete={remove}
            />
          ))
        )}
      </section>
    </section>
  );
}

function WatchlistRow(props: {
  item: WatchlistItem;
  quote?: MarketQuote | { error: string };
  refreshing: boolean;
  onDraftChange: (id: number, patch: Partial<WatchlistItem>) => void;
  onRefresh: (item: WatchlistItem) => void;
  onSave: (item: WatchlistItem) => void;
  onDelete: (item: WatchlistItem) => void;
}) {
  const quote = props.quote;
  const quoteError = quote && "error" in quote ? quote.error : "";
  const quoteValue = quote && !("error" in quote) ? quote : null;
  return (
    <div className="grid gap-3 border-t border-slate-100 px-4 py-4 text-sm md:grid-cols-[1fr_120px_120px_1.5fr_1.5fr_auto]">
      <div>
        <Link className="font-medium text-slate-900" to={`/stocks/${encodeURIComponent(props.item.symbol)}`}>
          {props.item.symbol}
        </Link>
        <div className="mt-1 text-xs text-slate-500">{quoteValue?.provider ? `来源 ${quoteValue.provider}` : "等待行情"}</div>
      </div>
      <div>
        <div className="text-xs text-slate-500">最新价</div>
        <div className="mt-1 font-medium text-slate-900">{quoteValue ? formatNumber(quoteValue.price) : quoteError || "-"}</div>
      </div>
      <div>
        <div className="text-xs text-slate-500">涨跌幅</div>
        <div className="mt-1 font-medium text-slate-900">{quoteValue?.change_percent === undefined ? "-" : `${formatNumber(quoteValue.change_percent)}%`}</div>
      </div>
      <label className="flex flex-col gap-1">
        <span className="text-xs text-slate-500">排序</span>
        <input
          aria-label={`排序 ${props.item.symbol}`}
          className="rounded border border-slate-300 px-2 py-1"
          value={props.item.sort_order}
          onChange={(event) => props.onDraftChange(props.item.id, { sort_order: Number(event.target.value) || 0 })}
        />
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-xs text-slate-500">标签</span>
        <input
          aria-label={`标签 ${props.item.symbol}`}
          className="rounded border border-slate-300 px-2 py-1"
          value={props.item.tags.join(",")}
          onChange={(event) => props.onDraftChange(props.item.id, { tags: splitTags(event.target.value) })}
        />
      </label>
      <div className="flex flex-col gap-2">
        <label className="flex flex-col gap-1">
          <span className="text-xs text-slate-500">备注</span>
          <input
            aria-label={`备注 ${props.item.symbol}`}
            className="rounded border border-slate-300 px-2 py-1"
            value={props.item.note}
            onChange={(event) => props.onDraftChange(props.item.id, { note: event.target.value })}
          />
        </label>
        <div className="flex gap-2">
          <Button aria-label={`刷新 ${props.item.symbol}`} loading={props.refreshing} onClick={() => props.onRefresh(props.item)}>
            刷新 {props.item.symbol}
          </Button>
          <Button aria-label={`保存 ${props.item.symbol}`} onClick={() => props.onSave(props.item)}>保存 {props.item.symbol}</Button>
          <Button aria-label={`删除 ${props.item.symbol}`} danger onClick={() => props.onDelete(props.item)}>删除 {props.item.symbol}</Button>
        </div>
      </div>
    </div>
  );
}

async function loadQuotes(items: WatchlistItem[]): Promise<Record<string, MarketQuote | { error: string }>> {
  const entries = await Promise.all(items.map(async (item) => [item.symbol, await loadQuote(item.symbol)] as const));
  return Object.fromEntries(entries);
}

async function loadQuote(symbol: string): Promise<MarketQuote | { error: string }> {
  try {
    return await marketQuote(symbol);
  } catch (error) {
    return { error: error instanceof Error ? error.message : "quote_error" };
  }
}

function splitTags(value: string): string[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 2 }).format(value);
}

function NewsExternalLink(props: { url?: string }) {
  const [error, setError] = useState<string | null>(null);

  if (!props.url) {
    return null;
  }
  const url = props.url;

  const openLink = async () => {
    setError(null);
    try {
      await openExternalURL(url);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "打开新闻外链失败");
    }
  };

  return (
    <div className="mt-2 flex flex-col gap-2">
      <div className="break-all text-xs text-slate-500">{url}</div>
      <Button aria-label="打开新闻外链" size="small" onClick={openLink}>
        打开
      </Button>
      {error ? <span className="text-xs text-red-600">{error}</span> : null}
    </div>
  );
}

function normalizeNewsItems(items: NewsItem[]) {
  return items.filter((item) => isHTTPSURL(item.url));
}

async function loadStockDetailState(normalizedSymbol: string, period: string, adjust: string): Promise<StockDetailViewState> {
  const [quote, kline, indicators, news] = await Promise.all([
    marketQuote(normalizedSymbol),
    marketKline({ symbol: normalizedSymbol, period, adjust, limit: 120 }),
    marketIndicators({
      symbol: normalizedSymbol,
      period,
      adjust,
      limit: 120,
      indicators: ["ma", "rsi", "macd"],
    }),
    newsList({ symbol: normalizedSymbol, limit: 20 }),
  ]);
  return {
    quote,
    kline: kline.items,
    indicators,
    news: normalizeNewsItems(news.items),
  };
}

function StockDetailRoute() {
  return <StockDetailPage />;
}

function KlinePanel(props: { items: MarketKlineItem[] }) {
  return (
    <DashboardList title="K 线图" emptyText="暂无 K 线数据">
      {props.items.length > 0 ? (
        <div className="border-t border-slate-100 px-4 py-4">
          <KlineChart items={props.items} />
          <table className="mt-4 w-full table-fixed text-left text-sm">
            <thead className="text-xs text-slate-500">
              <tr>
                <th className="py-2 font-medium">交易日</th>
                <th className="py-2 font-medium">开盘</th>
                <th className="py-2 font-medium">最高</th>
                <th className="py-2 font-medium">最低</th>
                <th className="py-2 font-medium">收盘</th>
              </tr>
            </thead>
            <tbody>
              {props.items.slice(-5).map((item) => (
                <tr key={`${item.trade_date}-${item.period}-${item.adjust}`} className="border-t border-slate-100">
                  <td className="py-2">{item.trade_date}</td>
                  <td className="py-2">{formatNumber(item.open)}</td>
                  <td className="py-2">{formatNumber(item.high)}</td>
                  <td className="py-2">{formatNumber(item.low)}</td>
                  <td className="py-2">{formatNumber(item.close)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </DashboardList>
  );
}

function IndicatorPanel(props: { indicators: Record<string, unknown> }) {
  const rows = indicatorRows(props.indicators);
  return (
    <DashboardList title="技术指标" emptyText="暂无技术指标">
      {rows.map((row) => (
        <div key={row.name} className="grid grid-cols-[1fr_auto] border-t border-slate-100 px-4 py-3 text-sm">
          <span className="font-medium text-slate-700">{row.name}</span>
          <span className="text-slate-900">{row.value}</span>
        </div>
      ))}
    </DashboardList>
  );
}

function NewsPanel(props: { items: NewsItem[] }) {
  return (
    <DashboardList title="相关新闻" emptyText="暂无相关新闻">
      {props.items.map((item) => (
        <article key={item.id} className="border-t border-slate-100 px-4 py-3 text-sm">
          <div className="font-medium text-slate-900">{item.title}</div>
          <div className="mt-1 text-xs text-slate-500">
            {[item.source, item.published_at].filter(Boolean).join(" · ")}
          </div>
          {item.summary ? <p className="mt-2 text-slate-600">{item.summary}</p> : null}
          <NewsExternalLink url={item.url} />
        </article>
      ))}
    </DashboardList>
  );
}

function isHTTPSURL(value?: string) {
  if (!value) {
    return false;
  }
  try {
    return new URL(value).protocol === "https:";
  } catch {
    return false;
  }
}

function indicatorRows(indicators: Record<string, unknown>) {
  return Object.entries(indicators).flatMap(([key, value]) => flattenIndicatorValue(key, value));
}

function flattenIndicatorValue(name: string, value: unknown): Array<{ name: string; value: string }> {
  if (Array.isArray(value)) {
    const latest = latestPrimitive(value);
    return latest === null ? [] : [{ name, value: formatIndicatorValue(latest) }];
  }
  if (typeof value === "number" || typeof value === "string" || typeof value === "boolean") {
    return [{ name, value: formatIndicatorValue(value) }];
  }
  if (value && typeof value === "object") {
    return Object.entries(value).flatMap(([key, child]) => flattenIndicatorValue(`${name}.${key}`, child));
  }
  return [];
}

function latestPrimitive(values: unknown[]) {
  for (let index = values.length - 1; index >= 0; index -= 1) {
    const value = values[index];
    if (typeof value === "number" || typeof value === "string" || typeof value === "boolean") {
      return value;
    }
  }
  return null;
}

function formatIndicatorValue(value: number | string | boolean) {
  return typeof value === "number" ? formatNumber(value) : String(value);
}

function SchedulerRoute() {
  const [state, setState] = useState<SchedulerViewState | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [actionPending, setActionPending] = useState<string | null>(null);
  const [selectedRun, setSelectedRun] = useState<SchedulerRun | null>(null);
  const [runFilters, setRunFilters] = useState({
    jobId: "0",
    status: "",
    triggerType: "",
  });
  const today = formatLocalDate(new Date());
  const [backfillForm, setBackfillForm] = useState({
    jobId: "",
    dateFrom: today,
    dateTo: today,
    symbols: "",
  });
  const [refreshForm, setRefreshForm] = useState({
    symbol: "",
    dataType: "all" as "quote" | "kline" | "news" | "all",
  });
  const [jobForm, setJobForm] = useState<SchedulerJobFormState>(() => defaultJobForm());

  const load = (jobId: string) =>
    Promise.all([
      schedulerStatus(),
      schedulerJobsList(),
      schedulerRunsList({ job_id: Number(jobId) || 0, limit: 50 }),
      schedulerJobTypes(),
      providersStatus(),
    ]).then(([status, jobs, runs, types, providers]) => ({
      status,
      jobs: jobs.items,
      runs: runs.items,
      types: types.items,
      providers: providers.items,
    }));

  useEffect(() => {
    let active = true;
    load(runFilters.jobId)
      .then((value) => {
        if (active) {
          setState(value);
          setLoadError(null);
        }
      })
      .catch((cause: unknown) => {
        if (active) {
          setLoadError(cause instanceof Error ? cause.message : "读取调度任务失败");
        }
      });
    return () => {
      active = false;
    };
  }, [runFilters.jobId]);

  const refresh = (jobId = runFilters.jobId) =>
    load(jobId).then((value) => {
      setState(value);
      setLoadError(null);
    });

  const runAction = (key: string, action: () => Promise<string>) => {
    setActionPending(key);
    setActionError(null);
    action()
      .then((message) => {
        setActionMessage(message);
      })
      .catch((cause: unknown) => {
        setActionError(cause instanceof Error ? cause.message : "调度操作失败");
      })
      .finally(() => {
        setActionPending(null);
      });
  };

  const runNow = (job: SchedulerJob) => {
    runAction(`run-${job.id}`, () =>
      schedulerJobsRunNow({
        id: job.id,
        target_date: today,
        ignore_trade_window: true,
      }).then((run) => refresh().then(() => `已入队 ${run.run_key}`)),
    );
  };

  const saveJob = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const validationError = validateJobForm(jobForm);
    if (validationError) {
      setActionError(validationError);
      return;
    }
    const payload = jobPayloadFromForm(jobForm);
    runAction("save-job", () =>
      schedulerJobsSave(payload).then((job) =>
        refresh().then(() => {
          setJobForm(defaultJobForm());
          return `${job.name || `任务 ${job.id}`} 已保存`;
        }),
      ),
    );
  };

  const editJob = (job: SchedulerJob) => {
    setJobForm(jobFormFromJob(job));
    setActionMessage(`正在编辑 ${job.name || `任务 ${job.id}`}`);
    setActionError(null);
  };

  const selectJobType = (cronType: string) => {
    const jobType = state?.types.find((item) => item.cron_type === cronType);
    setJobForm((current) => ({
      ...current,
      cronType,
      cronExpr: jobType?.default_cron_expr || current.cronExpr,
      tradeWindow: jobType?.default_window || current.tradeWindow,
      market: jobType?.market || current.market,
    }));
  };

  const setEnabled = (job: SchedulerJob, enabled: boolean) => {
    runAction(`enabled-${job.id}`, () =>
      schedulerJobsSetEnabled(job.id, enabled).then(() => refresh().then(() => `${job.name || `任务 ${job.id}`} 已${enabled ? "启用" : "停用"}`)),
    );
  };

  const deleteJob = (job: SchedulerJob) => {
    const name = job.name || `任务 ${job.id}`;
    if (!window.confirm(`确认删除 ${name}？`)) {
      return;
    }
    runAction(`delete-${job.id}`, () => schedulerJobsDelete(job.id).then(() => refresh().then(() => `${name} 已删除`)));
  };

  const changeRunJob = (jobId: string) => {
    setRunFilters((current) => ({ ...current, jobId }));
    setSelectedRun(null);
  };

  const viewRunDetail = (run: SchedulerRun) => {
    if (!run.id) {
      setSelectedRun(run);
      return;
    }
    setSelectedRun(null);
    runAction(`run-detail-${run.id}`, () =>
      schedulerRunsGet(run.id as number).then((detail) => {
        setSelectedRun(detail);
        return `已读取 ${detail.run_key}`;
      }),
    );
  };

  const submitBackfill = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const jobId = Number(backfillForm.jobId);
    const symbols = backfillForm.symbols
      .split(/[,\s]+/)
      .map((symbol) => symbol.trim())
      .filter(Boolean);
    const rangeError = validateBackfillRange(backfillForm.dateFrom, backfillForm.dateTo);
    if (!jobId) {
      setActionMessage(null);
      setActionError("请选择需要补偿的调度任务");
      return;
    }
    if (rangeError) {
      setActionMessage(null);
      setActionError(rangeError);
      return;
    }
    if (symbols.length > 30) {
      setActionMessage(null);
      setActionError("单次补偿最多支持 30 个 symbol");
      return;
    }
    runAction("backfill", () =>
      schedulerJobsBackfill({
        id: jobId,
        dateFrom: backfillForm.dateFrom,
        dateTo: backfillForm.dateTo,
        symbols,
      }).then((result) => refresh().then(() => `已生成 ${result.items.length} 条补偿执行记录`)),
    );
  };

  const submitRefreshSymbol = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const symbol = refreshForm.symbol.trim();
    if (!symbol) {
      setActionError("请填写需要刷新的股票代码");
      return;
    }
    runAction("refresh-symbol", () =>
      schedulerRefreshSymbol({
        symbol,
        data_type: refreshForm.dataType,
        target_date: today,
      }).then((result) => refresh().then(() => `已生成 ${result.items.length} 条单股刷新执行记录`)),
    );
  };

  if (loadError && !state) {
    return <Alert title="任务调度读取失败" description={loadError} type="error" showIcon />;
  }
  if (!state) {
    return <Alert title="正在读取任务调度" description={<Spin size="small" />} type="info" showIcon />;
  }

  return (
    <section className="flex flex-col gap-5">
      <div className="flex flex-col gap-1">
        <Typography.Title level={2} className="m-0">
          任务调度
        </Typography.Title>
        <Typography.Text type="secondary">数据刷新、启动补偿和手动补偿的本地执行状态。</Typography.Text>
      </div>
      {actionMessage ? <Alert title={actionMessage} type="success" showIcon /> : null}
      {actionError ? <Alert title={actionError} type="error" showIcon /> : null}
      {loadError ? <Alert title="任务调度刷新失败" description={loadError} type="warning" showIcon /> : null}
      <div className="grid gap-3 md:grid-cols-5">
        <Metric label="任务总数" value={state.status.jobs_total} />
        <Metric label="启用任务" value={state.status.jobs_enabled} />
        <Metric label="排队中" value={state.status.queued_runs} />
        <Metric label="运行中" value={state.status.running_runs} />
        <Metric label="失败记录" value={state.status.failed_runs} />
      </div>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            任务配置
          </Typography.Title>
        </div>
        <SchedulerJobEditor
          value={jobForm}
          types={state.types}
          actionPending={actionPending}
          onChange={setJobForm}
          onSelectType={selectJobType}
          onSubmit={saveJob}
          onReset={() => setJobForm(defaultJobForm())}
        />
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            调度任务
          </Typography.Title>
          <Typography.Text type="secondary">{state.types.length} 类任务类型</Typography.Text>
        </div>
        <SchedulerJobTable
          jobs={state.jobs}
          providers={state.providers}
          actionPending={actionPending}
          onRunNow={runNow}
          onEdit={editJob}
          onSetEnabled={setEnabled}
          onDelete={deleteJob}
        />
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            手动补偿
          </Typography.Title>
        </div>
        <SchedulerBackfillDialog value={backfillForm} jobs={state.jobs} actionPending={actionPending} onChange={setBackfillForm} onSubmit={submitBackfill} />
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <div className="border-b border-slate-200 px-4 py-3">
          <Typography.Title level={4} className="m-0">
            单股刷新
          </Typography.Title>
        </div>
        <form className="grid gap-3 px-4 py-4 md:grid-cols-[1.4fr_1fr_auto]" onSubmit={submitRefreshSymbol}>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">股票代码</span>
            <input
              className="h-8 rounded border border-slate-300 px-2"
              placeholder="600000.SH"
              value={refreshForm.symbol}
              onChange={(event) => setRefreshForm((current) => ({ ...current, symbol: event.target.value }))}
            />
          </label>
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-xs text-slate-500">数据类型</span>
            <select
              className="h-8 rounded border border-slate-300 px-2"
              value={refreshForm.dataType}
              onChange={(event) => setRefreshForm((current) => ({ ...current, dataType: event.target.value as typeof current.dataType }))}
            >
              <option value="all">全部</option>
              <option value="quote">行情</option>
              <option value="kline">K 线</option>
              <option value="news">新闻</option>
            </select>
          </label>
          <Button className="self-end" htmlType="submit" loading={actionPending === "refresh-symbol"}>
            刷新单股
          </Button>
        </form>
      </section>
      <section className="rounded-md border border-slate-200 bg-white">
        <SchedulerRunList
          jobs={state.jobs}
          runs={state.runs}
          filters={runFilters}
          selectedRun={selectedRun}
          actionPending={actionPending}
          onChangeJob={changeRunJob}
          onChangeStatus={(status) => setRunFilters((current) => ({ ...current, status }))}
          onChangeTriggerType={(triggerType) => setRunFilters((current) => ({ ...current, triggerType }))}
          onViewDetail={viewRunDetail}
        />
      </section>
    </section>
  );
}

function Metric(props: { label: string; value: number; suffix?: string }) {
  return (
    <div className="rounded-md border border-slate-200 bg-white px-4 py-3">
      <div className="text-xs text-slate-500">{props.label}</div>
      <div className="mt-1 text-2xl font-semibold text-slate-900">
        {formatNumber(props.value)}
        {props.suffix ?? ""}
      </div>
    </div>
  );
}

function formatLocalDate(value: Date) {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, "0");
  const day = String(value.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function uniqueValues(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort();
}

function validateBackfillRange(dateFrom: string, dateTo: string) {
  if (!dateFrom || !dateTo) {
    return "请选择补偿日期范围";
  }
  const start = new Date(`${dateFrom}T00:00:00`);
  const end = new Date(`${dateTo}T00:00:00`);
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
    return "补偿日期格式不正确";
  }
  if (start > end) {
    return "开始日期不能晚于结束日期";
  }
  const oneDayMs = 24 * 60 * 60 * 1000;
  if ((end.getTime() - start.getTime()) / oneDayMs > 29) {
    return "单次补偿最多覆盖 30 个自然日";
  }
  return "";
}

function defaultJobForm(): SchedulerJobFormState {
  return {
    id: 0,
    name: "",
    cronType: "cn_a_share_quote_refresh",
    cronExpr: "30 9 * * 1-5",
    enabled: true,
    market: "CN",
    timezone: "Asia/Shanghai",
    tradeWindow: "09:30-15:00",
    symbols: "",
    paramsJSON: "{}",
    catchupEnabled: true,
    catchupMaxDays: "5",
    timeoutSeconds: "120",
  };
}

function jobFormFromJob(job: SchedulerJob): SchedulerJobFormState {
  return {
    id: job.id,
    name: job.name || "",
    cronType: job.cron_type,
    cronExpr: job.cron_expr || "",
    enabled: Boolean(job.enabled),
    market: job.market || "CN",
    timezone: job.timezone || "Asia/Shanghai",
    tradeWindow: job.trade_window || "",
    symbols: symbolsFromScopeJSON(job.scope_json || ""),
    paramsJSON: job.params_json || "{}",
    catchupEnabled: Boolean(job.catchup_enabled),
    catchupMaxDays: String(job.catchup_max_days ?? 5),
    timeoutSeconds: String(job.timeout_seconds ?? 120),
  };
}

function validateJobForm(form: SchedulerJobFormState) {
  if (!form.name.trim()) {
    return "请填写任务名称";
  }
  if (!form.cronType.trim()) {
    return "请选择任务类型";
  }
  if (!form.cronExpr.trim()) {
    return "请填写 cron 表达式";
  }
  const params = parseJSON(form.paramsJSON);
  if (!params.ok) {
    return "参数 JSON 格式不正确";
  }
  const catchupMaxDays = Number(form.catchupMaxDays);
  if (!Number.isInteger(catchupMaxDays) || catchupMaxDays < 0 || catchupMaxDays > 30) {
    return "补偿天数必须是 0 到 30 的整数";
  }
  const timeoutSeconds = Number(form.timeoutSeconds);
  if (!Number.isInteger(timeoutSeconds) || timeoutSeconds <= 0 || timeoutSeconds > 3600) {
    return "超时秒数必须是 1 到 3600 的整数";
  }
  const symbols = splitSymbols(form.symbols);
  if (symbols.length > 30) {
    return "单个任务最多配置 30 个 symbol";
  }
  return "";
}

function jobPayloadFromForm(form: SchedulerJobFormState): SchedulerJobPayload {
  const symbols = splitSymbols(form.symbols);
  const params = parseJSON(form.paramsJSON);
  return {
    id: form.id,
    name: form.name.trim(),
    cron_type: form.cronType.trim(),
    cron_expr: form.cronExpr.trim(),
    enabled: form.enabled,
    market: form.market.trim() || "CN",
    timezone: form.timezone.trim() || "Asia/Shanghai",
    trade_window: form.tradeWindow.trim(),
    scope_json: JSON.stringify(symbols.length > 0 ? { symbols } : { market: form.market.trim() || "CN" }),
    params_json: JSON.stringify(params.value),
    catchup_enabled: form.catchupEnabled,
    catchup_max_days: Number(form.catchupMaxDays),
    timeout_seconds: Number(form.timeoutSeconds),
  };
}

function splitSymbols(value: string) {
  return value
    .split(/[,\s]+/)
    .map((symbol) => symbol.trim())
    .filter(Boolean);
}

function symbolsFromScopeJSON(scopeJSON: string) {
  const parsed = parseJSON(scopeJSON);
  if (!parsed.ok || typeof parsed.value !== "object" || parsed.value === null || !("symbols" in parsed.value)) {
    return "";
  }
  const symbols = (parsed.value as { symbols?: unknown }).symbols;
  return Array.isArray(symbols) ? symbols.filter((symbol): symbol is string => typeof symbol === "string").join(", ") : "";
}

function parseJSON(value: string): { ok: true; value: unknown } | { ok: false; value: null } {
  try {
    return { ok: true, value: value.trim() ? JSON.parse(value) : {} };
  } catch {
    return { ok: false, value: null };
  }
}

function DashboardList(props: { title: string; emptyText: string; children: ReactNode }) {
  const items = Array.isArray(props.children) ? props.children.filter(Boolean) : props.children ? [props.children] : [];
  return (
    <section className="rounded-md border border-slate-200 bg-white">
      <div className="border-b border-slate-200 px-4 py-3">
        <Typography.Title level={4} className="m-0">
          {props.title}
        </Typography.Title>
      </div>
      {items.length > 0 ? props.children : <div className="px-4 py-8 text-sm text-slate-500">{props.emptyText}</div>}
    </section>
  );
}
