import { invoke as tauriInvoke, type InvokeArgs } from "@tauri-apps/api/core";
import { open } from "@tauri-apps/plugin-dialog";

type CoreEnvelope<T> = {
  code: number;
  message: string;
  data: T;
  traceId?: string;
  requestId?: string;
};

export class CoreClientError extends Error {
  code?: number;
  requestId?: string;
  traceId?: string;

  constructor(message: string, options: { code?: number; requestId?: string; traceId?: string } = {}) {
    super(message);
    this.name = "CoreClientError";
    this.code = options.code;
    this.requestId = options.requestId;
    this.traceId = options.traceId;
  }
}

export type CoreHealth = {
  status: string;
  version: string;
  dbStatus?: string;
};

export type AppBootStatus = {
  ready: boolean;
  progress: number;
  currentStepId: string;
  steps: Array<{
    id: string;
    index: number;
    title: string;
    status: "completed" | "running" | "pending" | "failed";
    badgeText: string;
  }>;
  taskDetail: {
    taskId: string;
    elapsed: string;
    currentStage: string;
    remaining: string;
  };
  logs: Array<{
    id: string;
    time: string;
    status: "success" | "running" | "pending" | "error";
    message: string;
  }>;
};

export type DashboardSummary = {
  watchlist: {
    up_count: number;
    down_count: number;
    flat_count: number;
  };
  recent_reports: Array<{
    id: number;
    task_id?: string;
    symbol?: string;
    title: string;
    analysis_type?: string;
    model_name?: string;
    prompt_template_id?: number;
    risk_summary?: string;
    created_at?: string;
    updated_at?: string;
  }>;
  recent_tasks: Array<{
    id: string;
    type?: string;
    task_type?: string;
    status: string;
    title: string;
    progress?: number;
    error_message?: string;
    created_at?: string;
    updated_at?: string;
  }>;
  market_news: Array<{
    title: string;
    source?: string;
    summary?: string;
    url?: string;
    published_at?: string;
  }>;
  risk_tips: string[];
  provider_statuses: Array<{
    name: string;
    source: string;
    available: boolean;
    last_error?: string;
  }>;
};

export type StockSearchResult = {
  symbol: string;
  name: string;
  code: string;
  market: string;
  exchange: string;
};

export type StockProfile = StockSearchResult & {
  industry?: string;
  concepts?: string[];
  list_date?: string;
  status?: string;
  full_name?: string;
};

export type MarketQuote = {
  symbol: string;
  price: number;
  change_amount?: number;
  change_percent?: number;
  open?: number;
  high?: number;
  low?: number;
  pre_close?: number;
  volume?: number;
  amount?: number;
  turnover_rate?: number;
  pe?: number;
  pb?: number;
  total_market_cap?: number;
  float_market_cap?: number;
  quote_time?: string;
  provider?: string;
};

export type MarketKlineItem = {
  symbol: string;
  period: string;
  adjust: string;
  trade_date: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume?: number;
  amount?: number;
  provider?: string;
};

export type MarketKlinePayload = {
  symbol: string;
  period: string;
  adjust: string;
  limit: number;
};

export type MarketKlineResult = {
  items: MarketKlineItem[];
};

export type MarketIndicatorsPayload = MarketKlinePayload & {
  indicators: string[];
};

export type MarketIndicatorsResult = {
  symbol: string;
  period: string;
  adjust: string;
  indicators: Record<string, unknown>;
};

export type NewsItem = {
  id: number;
  source?: string;
  title: string;
  url?: string;
  summary?: string;
  content_hash?: string;
  published_at?: string;
  symbols?: string[];
  tags?: string[];
};

export type NewsListPayload = {
  symbol: string;
  limit: number;
};

export type NewsMarketPayload = {
  market: string;
  limit: number;
};

export type NewsListResult = {
  items: NewsItem[];
};

export type NewsStatsResult = {
  total_count: number;
  source_count: number;
  latest_published_at?: string;
  sentiment_summary: string;
};

export type NewsHotTopicsResult = {
  industries: Array<{
    name: string;
    count: number;
  }>;
  mentioned_stocks: Array<{
    symbol: string;
    count: number;
  }>;
  updated_at?: string;
};

export type OpenExternalURLResult = {
  ok: boolean;
};

export type AIConfig = {
  id: number;
  name: string;
  provider: string;
  base_url: string;
  api_key_ref: string;
  masked_api_key: string;
  has_api_key: boolean;
  model_name: string;
  temperature: number;
  max_tokens: number;
  timeout_seconds: number;
  stream_enabled: boolean;
  is_default: boolean;
  created_at?: string;
  updated_at?: string;
};

export type AIConfigList = {
  items: AIConfig[];
};

export type AIConfigSavePayload = AIConfig & {
  api_key?: string;
};

export type AIConfigSaveResult = {
  config: AIConfig;
};

export type AIConfigTestPayload = {
  id: number;
  api_key_ref: string;
};

export type AIConfigTestResult = {
  ok: boolean;
  provider: string;
  model: string;
  message: string;
  duration_ms: number;
};

export type AIConfigDeletePayload = {
  id: number;
};

export type AIConfigDeleteResult = {
  deleted_id: number;
};

export type PromptTemplateType = "system" | "stock_full" | "technical" | "fundamental" | "news" | "custom";

export type PromptTemplate = {
  id: number;
  key?: string;
  name: string;
  type: PromptTemplateType;
  description: string;
  content: string;
  variables: string[];
  is_builtin: boolean;
  builtin_locked?: boolean;
  version?: number;
  checksum?: string;
  source?: "builtin" | "user" | string;
  created_at?: string;
  updated_at?: string;
};

export type PromptTemplatesList = {
  items: PromptTemplate[];
};

export type PromptTemplateCreatePayload = {
  name: string;
  type: PromptTemplateType;
  description: string;
  content: string;
};

export type PromptTemplateUpdatePayload = PromptTemplateCreatePayload & {
  id: number;
};

export type UserPositionPayload = {
  cost_price: number;
  shares: number;
  risk_level: string;
};

export type AnalysisTaskCreatePayload = {
  symbol: string;
  analysis_type: "stock_full" | "technical";
  ai_config_id: number;
  api_key_ref: string;
  prompt_template_id: number;
  retry_of_task_id?: string;
  user_position?: UserPositionPayload | null;
};

export type AnalysisTaskResult = {
  task_id: string;
  status: string;
};

export type AnalysisTaskSubscribeResult = {
  emitted: number;
  last_event_id: number;
};

export type TaskItem = {
  id: string;
  type: string;
  status: string;
  title: string;
  progress: number;
  report_id?: number;
  error_message?: string;
  started_at?: string;
  finished_at?: string;
  created_at?: string;
  updated_at?: string;
};

export type TaskListResult = {
  items: TaskItem[];
};

export type TaskEventItem = {
  id: number;
  task_id?: string;
  event: string;
  data: Record<string, unknown>;
  created_at?: string;
};

export type TaskEventsResult = {
  items: TaskEventItem[];
};

export type TaskLogLevel = "INFO" | "WARN" | "ERROR";

export type TaskLogRow = {
  id: number;
  task_id: string;
  time: string;
  timestamp: string;
  level: TaskLogLevel;
  module: string;
  stage: string;
  message: string;
  code?: string;
  provider?: string;
  model?: string;
  symbol?: string;
  duration_ms?: number;
  retryable: boolean;
};

export type TaskLogListPayload = {
  taskId: string;
  level?: "" | TaskLogLevel;
  module?: string;
  stage?: string;
  keyword?: string;
  onlyError?: boolean;
  afterId?: number;
  limit?: number;
};

export type TaskLogListResult = {
  rows: TaskLogRow[];
  next_after_id: number;
  has_more: boolean;
};

export type TaskLogDetail = TaskLogRow & {
  raw_json: string;
};

export type TaskLogSummary = {
  title: string;
  task_id: string;
  task_type: string;
  stock?: string;
  model?: string;
  started_at: string;
  duration: string;
  request_id: string;
  trace_id: string;
  status: string;
};

export type TaskLogDiagnosis = {
  task_id: string;
  error_code?: string;
  error_stage?: string;
  summary: string;
  causes: string[];
  suggestions: string[];
  retryable: boolean;
  request_id?: string;
  trace_id?: string;
  source_log_id?: number;
};

export type TaskLogContextSummary = {
  task_id: string;
  stock?: string;
  analysis_type?: string;
  model?: string;
  prompt_template?: string;
  quote_status?: string;
  kline_status?: string;
  indicator_status?: string;
  news_status?: string;
  user_position?: string;
  data_updated_at?: string;
  report_created_at?: string;
  raw_snapshot_brief?: string;
};

export type TaskLogsExportResult = {
  file_path: string;
  file_name: string;
};

export type AnalysisReport = {
  id: number;
  task_id: string;
  symbol: string;
  title: string;
  analysis_type?: string;
  model_name?: string;
  prompt_template_id?: number;
  content_markdown?: string;
  risk_summary?: string;
  favorite?: boolean;
  created_at?: string;
  updated_at?: string;
};

export type ReportListResult = {
  items: AnalysisReport[];
};

export type ReportStatsCount = {
  name: string;
  count: number;
};

export type ReportStatsResult = {
  total: number;
  unique_symbols: number;
  latest_created_at?: string;
  analysis_types: ReportStatsCount[];
  top_models: ReportStatsCount[];
};

export type ReportBatchDeleteResult = {
  ids: number[];
};

export type ReportExportResult = {
  saved: boolean;
  file_path: string;
  file_name: string;
};

export type WatchlistItem = {
  id: number;
  symbol: string;
  sort_order: number;
  tags: string[] | null;
  note: string | null;
  created_at?: string;
  updated_at?: string;
  name?: string;
  code?: string;
  market?: string;
  exchange?: string;
  industry?: string;
  concepts?: string[];
  list_date?: string;
  status?: string;
  full_name?: string;
  quote?: MarketQuote | null;
  trend_points?: number[];
};

export type WatchlistList = {
  items: WatchlistItem[];
};

export type WatchlistCreatePayload = {
  symbol: string;
  sort_order: number;
  tags: string[];
  note: string;
};

export type WatchlistUpdatePayload = {
  id: number;
  sort_order: number;
  tags: string[];
  note: string;
};

export type WatchlistRefreshPayload = {
  symbols: string[];
};

export type WatchlistRefreshResult = {
  accepted: boolean;
  total: number;
};

export type SchedulerJob = {
  id: number;
  name?: string;
  cron_type: string;
  cron_expr?: string;
  enabled?: boolean;
  market?: string;
  timezone?: string;
  trade_window?: string;
  scope_json?: string;
  params_json?: string;
  catchup_enabled?: boolean;
  catchup_max_days?: number;
  timeout_seconds?: number;
  last_status?: string;
  last_error?: string;
  created_at?: string;
  updated_at?: string;
};

export type SchedulerJobsList = {
  items: SchedulerJob[];
};

export type SchedulerJobPayload = Required<
  Pick<
    SchedulerJob,
    | "id"
    | "name"
    | "cron_type"
    | "cron_expr"
    | "enabled"
    | "market"
    | "timezone"
    | "trade_window"
    | "scope_json"
    | "params_json"
    | "catchup_enabled"
    | "catchup_max_days"
    | "timeout_seconds"
  >
>;

export type SchedulerJobType = {
  cron_type: string;
  label: string;
  default_cron_expr?: string;
  default_window?: string;
  market?: string;
  enabled?: boolean;
};

export type SchedulerJobTypes = {
  items: SchedulerJobType[];
};

export type SchedulerRun = {
  id?: number;
  job_id?: number;
  cron_type?: string;
  data_type?: string;
  period?: string;
  run_key: string;
  trigger_type: string;
  status: string;
  priority?: number;
  source?: string;
  target_date?: string;
  scope_key?: string;
  fetched_count?: number;
  written_count?: number;
  skipped_reason?: string;
  error_message?: string;
  started_at?: string;
  finished_at?: string;
  created_at?: string;
  updated_at?: string;
};

export type SchedulerTriggerPayload = {
  job_id: number;
  cron_type: string;
  scope_key: string;
  target_date: string;
};

export type SchedulerRunsListPayload = {
  job_id: number;
  limit: number;
};

export type SchedulerRunsList = {
  items: SchedulerRun[];
};

export type SchedulerRunNowPayload = {
  id: number;
  target_date: string;
  ignore_trade_window: boolean;
};

export type SchedulerBackfillPayload = {
  id: number;
  dateFrom: string;
  dateTo: string;
  symbols: string[];
};

export type SchedulerBackfillResult = {
  items: SchedulerRun[];
};

export type SchedulerStatus = {
  jobs_total: number;
  jobs_enabled: number;
  queued_runs: number;
  running_runs: number;
  failed_runs: number;
};

export type SchedulerRefreshSymbolPayload = {
  symbol: string;
  data_type: "quote" | "kline" | "news" | "all";
  target_date?: string;
  period?: string;
  adjust?: string;
  limit?: number;
};

export type SchedulerRefreshSymbolResult = {
  items: SchedulerRun[];
};

export type SettingItem = {
  key: string;
  value: string;
};

export type SettingsGetResult = {
  items: SettingItem[];
};

export type SettingsSetPayload = {
  items: SettingItem[];
  proxy_password?: string;
  proxy_credential_ref?: string;
  clear_proxy_credential?: boolean;
};

export type SettingsSetResult = {
  saved_keys: string[];
};

export type ProxyConnectionTestPayload = {
  target: string;
};

export type ProxyConnectionTestResult = {
  result: {
    ok: boolean;
    target: string;
    status_code: number;
    duration_ms: number;
    checked_at: string;
    message: string;
  };
};

export type AutostartState = {
  enabled: boolean;
};

export type WorkspaceResult = {
  path: string;
};

export type WorkspaceMigrationPlan = {
  current_path: string;
  target_path: string;
  can_migrate: boolean;
  reason: string;
  warnings: string[];
  source_size_bytes: number;
  available_space_bytes: number | null;
  target_exists: boolean;
  target_empty: boolean;
  will_create_target: boolean;
};

export type WorkspaceMigratePayload = {
  target_path: string;
  include_cache: boolean;
  create_backup: boolean;
};

export type WorkspaceMigrateResult = {
  path: string;
  migrated_files: string[];
  backup_path?: string | null;
};

export type WorkspaceOpenResult = {
  opened: boolean;
};

export type CacheStatsItem = {
  target: string;
  bytes: number;
  label?: string;
  cleanable?: boolean;
};

export type CacheStatsResult = {
  total_bytes: number;
  items: CacheStatsItem[];
};

export type CacheCleanResult = {
  cleaned_targets: string[];
};

export type NotificationItem = {
  id: number;
  type: string;
  level: string;
  title: string;
  content: string;
  source_type?: string;
  source_id?: string;
  route?: string;
  is_read: boolean;
  created_at: string;
  read_at?: string | null;
};

export type NotificationsListPayload = {
  unread_only: boolean;
  limit: number;
  offset: number;
};

export type NotificationsListResult = {
  items: NotificationItem[];
  total: number;
  limit: number;
  offset: number;
};

export type NotificationsUnreadCountResult = {
  count: number;
};

export type NotificationsMarkReadPayload = {
  ids: number[];
};

export type NotificationsActionResult = {
  ok: boolean;
};

export type SearchIndexStatus = {
  fts5_status: string;
  gse_status?: string;
  search_status: string;
  active_stock_batch_id?: string;
  active_document_batch_id?: string;
  running_rebuild_task_id?: string;
  stock_index_count?: number;
  report_index_count?: number;
  news_index_count?: number;
  watchlist_note_index_count?: number;
  last_rebuild_at?: string;
  tokenizer_name?: string;
  tokenizer_version?: string;
  dictionary_hash?: string;
};

export type SearchRebuildPayload = {
  scope: "all" | "stock" | "reports" | "news" | "watchlist_notes";
};

export type SearchRebuildResult = {
  task_id: string;
  scope?: string;
  stock_batch_id?: string;
  document_batch_id?: string;
};

export type DocumentSearchPayload = {
  keyword: string;
  symbols: string[];
  limit: number;
  offset: number;
  sort: string;
};

export type DocumentSearchItem = {
  doc_uid: string;
  doc_type: "report" | "news" | "watchlist_note";
  ref_id: string;
  symbol: string;
  title: string;
  summary: string;
  source: string;
  source_time: string;
  score: number;
  highlights: string[];
};

export type ProviderStatusItem = {
  name: string;
  source: string;
  available: boolean;
  last_error?: string;
};

export type ProvidersStatusResult = {
  items: ProviderStatusItem[];
};

export type DataSourceCredentialAuthType = "none" | "api_key" | "cookie" | "bearer_token" | "custom_header";

export type DataSourceCredentialStatus = "normal" | "not_configured" | "expired" | "expiring" | "limited" | "failed";

export type DataSourceCredentialProvider = {
  id: string;
  name: string;
  capability: string;
  status: DataSourceCredentialStatus;
  authType: DataSourceCredentialAuthType;
  iconType: string;
};

export type DataSourceCredentialConfig = {
  providerId: string;
  providerName: string;
  capability: string;
  authType: DataSourceCredentialAuthType;
  baseUrl: string;
  credentialStatus: DataSourceCredentialStatus;
  expiresAt?: string;
  timeoutSeconds: number;
  rateLimitPerMinute: number;
  maskedCredential: string;
  note?: string;
  lastTestResult?: DataSourceCredentialTestResult;
};

export type DataSourceCredentialTestTarget = {
  label: string;
  value: string;
};

export type DataSourceCredentialTestResult = {
  status: "success" | "failed" | "untested" | "limited";
  responseTimeMs?: number;
  testedAt?: string;
  messages: string[];
};

export type DataSourceCredentialOverview = {
  configuredCount: number;
  expiringSoonCount: number;
  expiredCount: number;
};

export type DataSourceCredentialHealthItem = {
  name: string;
  status: "normal" | "limited" | "failed";
  rateLimitText: string;
};

export type DataSourceCredentialOperationLog = {
  id: string;
  action: string;
  status: "success" | "failed";
  time: string;
};

export type DataSourceCredentialsListResult = {
  providers: DataSourceCredentialProvider[];
  configs: Record<string, DataSourceCredentialConfig>;
  selectedProviderId: string;
  testTargets: DataSourceCredentialTestTarget[];
  testResult: DataSourceCredentialTestResult;
  overview: DataSourceCredentialOverview;
  healthItems: DataSourceCredentialHealthItem[];
  operationLogs: DataSourceCredentialOperationLog[];
};

export type DataSourceCredentialSavePayload = {
  config: DataSourceCredentialConfig;
  credential: string;
};

export type DataSourceCredentialSaveResult = {
  config: DataSourceCredentialConfig;
};

export type DataSourceCredentialClearPayload = {
  providerId: string;
};

export type DataSourceCredentialClearResult = {
  config: DataSourceCredentialConfig;
};

export type DataSourceCredentialTestPayload = {
  providerId: string;
  target: string;
};

export type DataSourceCredentialTestResponse = {
  result: DataSourceCredentialTestResult;
};

export type UpdateCheckResult = {
  current_version: string;
  latest_version: string;
  has_new_version: boolean;
  action: "PROMPT_ONLY";
  download_url?: string;
  release_notes_url?: string;
};

export type ExportLogsResult = {
  file_path: string;
  file_name: string;
};

export type LogsOpenDirectoryResult = {
  opened: boolean;
};

async function invoke<T>(command: string, payload?: InvokeArgs): Promise<T> {
  try {
    return await tauriInvoke<T>(command, payload);
  } catch (cause) {
    throw normalizeCoreError(cause);
  }
}

function unwrapCoreResponse<T>(response: CoreEnvelope<T>): T {
  if (response.code !== 0) {
    throw new CoreClientError(sanitizeCoreErrorMessage(response.message || "本地核心服务调用失败"), {
      code: response.code,
      requestId: response.requestId,
      traceId: response.traceId,
    });
  }
  return response.data;
}

function normalizeCoreError(cause: unknown): CoreClientError {
  if (cause instanceof CoreClientError) {
    return cause;
  }
  const message = cause instanceof Error ? cause.message : typeof cause === "string" ? cause : "本地核心服务调用失败";
  return new CoreClientError(sanitizeCoreErrorMessage(message));
}

function sanitizeCoreErrorMessage(message: string): string {
  return message
    .replace(/(Authorization|Proxy-Authorization)\s*:\s*[^,\n\r;]+/gi, "$1: [REDACTED]")
    .replace(/(api[_-]?key|token|cookie|password|secret)=([^;&\s]+)/gi, "$1=[REDACTED]")
    .replace(/sk-[A-Za-z0-9_-]{6,}/g, "[REDACTED]");
}

/// 通过固定 Rust command 读取 Go core 健康状态，前端不接触 Go core 地址或 token。
export async function coreHealth(): Promise<CoreHealth> {
  const response = await invoke<CoreEnvelope<CoreHealth>>("core_health");
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取应用启动初始化状态，启动页不直接调用各业务接口。
export async function appBootStatus(): Promise<AppBootStatus> {
  const response = await invoke<CoreEnvelope<AppBootStatus>>("app_boot_status");
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取 Dashboard 汇总，首页只消费 Go core 聚合后的真实数据。
export async function dashboardSummary(): Promise<DashboardSummary> {
  const response = await invoke<CoreEnvelope<DashboardSummary>>("dashboard_summary");
  return normalizeDashboardSummary(unwrapCoreResponse(response));
}

function normalizeDashboardSummary(summary: DashboardSummary): DashboardSummary {
  return {
    ...summary,
    watchlist: summary.watchlist ?? { up_count: 0, down_count: 0, flat_count: 0 },
    recent_reports: Array.isArray(summary.recent_reports) ? summary.recent_reports : [],
    recent_tasks: Array.isArray(summary.recent_tasks) ? summary.recent_tasks : [],
    market_news: Array.isArray(summary.market_news) ? summary.market_news : [],
    risk_tips: Array.isArray(summary.risk_tips) ? summary.risk_tips : [],
    provider_statuses: Array.isArray(summary.provider_statuses) ? summary.provider_statuses : [],
  };
}

type RawTaskEventItem = {
  id: number;
  task_id?: string;
  event?: string;
  event_type?: string;
  data?: Record<string, unknown>;
  payload?: string | Record<string, unknown>;
  created_at?: string;
};

function normalizeTaskEventsResult(result: TaskEventsResult | { items?: RawTaskEventItem[] }): TaskEventsResult {
  return {
    items: (result.items ?? []).map(normalizeTaskEventItem),
  };
}

function normalizeTaskEventItem(item: TaskEventItem | RawTaskEventItem): TaskEventItem {
  const raw = item as RawTaskEventItem;
  return {
    id: raw.id,
    task_id: raw.task_id,
    event: raw.event ?? raw.event_type ?? "",
    data: raw.data ?? parseTaskEventPayload(raw.payload),
    created_at: raw.created_at,
  };
}

function parseTaskEventPayload(payload: RawTaskEventItem["payload"]): Record<string, unknown> {
  if (!payload) {
    return {};
  }
  if (typeof payload !== "string") {
    return payload;
  }
  try {
    const parsed = JSON.parse(payload) as unknown;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed as Record<string, unknown> : {};
  } catch {
    return {};
  }
}

/// 通过固定 Rust command 搜索股票，搜索结果来自 Go core Provider，不在前端构造假数据。
export async function stockSearch(keyword: string): Promise<StockSearchResult[]> {
  const response = await invoke<CoreEnvelope<StockSearchResult[]>>("stock_search", { keyword });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取股票基础资料，详情页和自选股共用 Go core 的 stocks 表数据。
export async function stockProfile(symbol: string): Promise<StockProfile> {
  const response = await invoke<CoreEnvelope<StockProfile>>("stock_profile", { symbol });
  return unwrapCoreResponse(response);
}

export type MarketQuoteOptions = {
  forceRefresh?: boolean;
};

/// 通过固定 Rust command 读取单只股票行情，用于自选股等页面展示真实 quote。
export async function marketQuote(symbol: string, options?: MarketQuoteOptions): Promise<MarketQuote> {
  const payload = options?.forceRefresh ? { symbol, forceRefresh: true } : { symbol };
  const response = await invoke<CoreEnvelope<MarketQuote>>("market_quote", payload);
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取 K 线数据，payload 与 Rust 白名单 command 保持一一对应。
export async function marketKline(payload: MarketKlinePayload): Promise<MarketKlineResult> {
  const response = await invoke<CoreEnvelope<MarketKlineResult>>(
    "market_kline",
    payload,
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取技术指标摘要，前端只展示后端计算结果。
export async function marketIndicators(
  payload: MarketIndicatorsPayload,
): Promise<MarketIndicatorsResult> {
  const response = await invoke<CoreEnvelope<MarketIndicatorsResult>>(
    "market_indicators",
    payload,
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取个股新闻，外链安全策略在页面层继续收口。
export async function newsList(payload: NewsListPayload): Promise<NewsListResult> {
  const response = await invoke<CoreEnvelope<NewsListResult>>("news_list", payload);
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取市场新闻，资讯中心不直接访问外部数据源。
export async function newsMarket(payload: NewsMarketPayload): Promise<NewsListResult> {
  const response = await invoke<CoreEnvelope<NewsListResult>>("news_market", payload);
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取资讯缓存统计，不在前端伪造情绪分类。
export async function newsStats(payload: NewsMarketPayload): Promise<NewsStatsResult> {
  const response = await invoke<CoreEnvelope<NewsStatsResult>>("news_stats", payload);
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取资讯缓存热点标签和提及股票统计。
export async function newsHotTopics(payload: NewsMarketPayload): Promise<NewsHotTopicsResult> {
  const response = await invoke<CoreEnvelope<NewsHotTopicsResult>>("news_hot_topics", payload);
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 打开 HTTPS 外链，前端不直接使用浏览器 window.open。
export async function openExternalURL(url: string): Promise<OpenExternalURLResult> {
  return invoke<OpenExternalURLResult>("open_external_url", { url });
}

/// 通过固定 Rust command 读取 AI 配置列表，只接收脱敏展示字段。
export async function aiConfigList(): Promise<AIConfigList> {
  const response = await invoke<CoreEnvelope<AIConfigList>>("ai_config_list");
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 保存 AI 配置；api_key 只作为一次性字段交给 Rust vault。
export async function aiConfigSave(payload: AIConfigSavePayload): Promise<AIConfigSaveResult> {
  const response = await invoke<CoreEnvelope<AIConfigSaveResult>>("ai_config_save", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 测试 AI 配置连通性，真实 Key 由 Rust 从本地 vault 读取。
export async function aiConfigTest(payload: AIConfigTestPayload): Promise<AIConfigTestResult> {
  const response = await invoke<CoreEnvelope<AIConfigTestResult>>("ai_config_test", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 删除 AI 配置，Rust 负责同步清理本地 vault 引用。
export async function aiConfigDelete(payload: AIConfigDeletePayload): Promise<AIConfigDeleteResult> {
  const response = await invoke<CoreEnvelope<AIConfigDeleteResult>>("ai_config_delete", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取 Prompt 模板列表。
export async function promptTemplatesList(): Promise<PromptTemplatesList> {
  const response = await invoke<CoreEnvelope<PromptTemplatesList>>(
    "prompt_templates_list",
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取单个 Prompt 模板。
export async function promptTemplatesGet(id: number): Promise<PromptTemplate> {
  const response = await invoke<CoreEnvelope<PromptTemplate>>(
    "prompt_templates_get",
    { id },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 创建 Prompt 模板。
export async function promptTemplatesCreate(
  payload: PromptTemplateCreatePayload,
): Promise<PromptTemplate> {
  const response = await invoke<CoreEnvelope<PromptTemplate>>(
    "prompt_templates_create",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 更新 Prompt 模板。
export async function promptTemplatesUpdate(
  payload: PromptTemplateUpdatePayload,
): Promise<PromptTemplate> {
  const response = await invoke<CoreEnvelope<PromptTemplate>>(
    "prompt_templates_update",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 删除 Prompt 模板。
export async function promptTemplatesDelete(id: number): Promise<{ id: number }> {
  const response = await invoke<CoreEnvelope<{ id: number }>>(
    "prompt_templates_delete",
    { id },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 创建分析任务，前端只传本地 vault 引用和任务参数。
export async function analysisTaskCreate(payload: AnalysisTaskCreatePayload): Promise<AnalysisTaskResult> {
  const response = await invoke<CoreEnvelope<AnalysisTaskResult>>("analysis_task_create", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 取消分析任务。
export async function analysisTaskCancel(taskID: string): Promise<AnalysisTaskResult> {
  const response = await invoke<CoreEnvelope<AnalysisTaskResult>>("analysis_task_cancel", { taskId: taskID });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 订阅分析任务事件，事件帧由 Rust 转发为 Tauri event。
export async function analysisTaskSubscribe(taskID: string, afterEventID: number): Promise<AnalysisTaskSubscribeResult> {
  return invoke<AnalysisTaskSubscribeResult>("analysis_task_subscribe", { taskId: taskID, afterEventId: afterEventID });
}

/// 通过固定 Rust command 读取任务历史列表。
export async function taskList(limit: number): Promise<TaskListResult> {
  const response = await invoke<CoreEnvelope<TaskListResult>>("task_list", { limit });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取任务详情。
export async function taskGet(taskID: string): Promise<TaskItem> {
  const response = await invoke<CoreEnvelope<TaskItem>>("task_get", { taskId: taskID });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 增量读取任务事件。
export async function taskEvents(taskID: string, afterEventID: number): Promise<TaskEventsResult> {
  const response = await invoke<CoreEnvelope<TaskEventsResult>>("task_events", { taskId: taskID, afterEventId: afterEventID });
  return normalizeTaskEventsResult(unwrapCoreResponse(response));
}

/// 通过固定 Rust command 读取任务结构化日志列表。
export async function taskLogsList(payload: TaskLogListPayload): Promise<TaskLogListResult> {
  const response = await invoke<CoreEnvelope<TaskLogListResult>>("task_logs_list", payload);
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取单条任务结构化日志详情。
export async function taskLogGet(id: number): Promise<TaskLogDetail> {
  const response = await invoke<CoreEnvelope<TaskLogDetail>>("task_log_get", { id });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取日志抽屉基础摘要。
export async function taskLogSummary(taskID: string): Promise<TaskLogSummary> {
  const response = await invoke<CoreEnvelope<TaskLogSummary>>("task_log_summary", { taskId: taskID });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取任务错误诊断。
export async function taskLogDiagnosis(taskID: string): Promise<TaskLogDiagnosis> {
  const response = await invoke<CoreEnvelope<TaskLogDiagnosis>>("task_log_diagnosis", { taskId: taskID });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取任务上下文摘要。
export async function taskLogContext(taskID: string): Promise<TaskLogContextSummary> {
  const response = await invoke<CoreEnvelope<TaskLogContextSummary>>("task_log_context", { taskId: taskID });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 将 Go core 生成的脱敏日志包写入用户授权目录。
export async function taskLogsExport(taskID: string, targetDir: string): Promise<TaskLogsExportResult> {
  return invoke<TaskLogsExportResult>("task_logs_export", { taskId: taskID, targetDir });
}

/// 通过固定 Rust command 读取报告历史列表。
export async function reportList(): Promise<ReportListResult> {
  const response = await invoke<CoreEnvelope<ReportListResult>>("report_list");
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取报告详情。
export async function reportGet(id: number): Promise<AnalysisReport> {
  const response = await invoke<CoreEnvelope<AnalysisReport>>("report_get", { id });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 删除报告。
export async function reportDelete(id: number): Promise<{ id: number }> {
  const response = await invoke<CoreEnvelope<{ id: number }>>("report_delete", { id });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 更新报告收藏状态。
export async function reportUpdate(id: number, favorite: boolean): Promise<AnalysisReport> {
  const response = await invoke<CoreEnvelope<AnalysisReport>>("report_update", { id, favorite });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取报告聚合统计。
export async function reportStats(): Promise<ReportStatsResult> {
  const response = await invoke<CoreEnvelope<ReportStatsResult>>("report_stats");
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 批量删除报告。
export async function reportBatchDelete(ids: number[]): Promise<ReportBatchDeleteResult> {
  const response = await invoke<CoreEnvelope<ReportBatchDeleteResult>>("report_batch_delete", { ids });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 打开系统保存对话框并导出报告 Markdown。
export async function reportExport(id: number): Promise<ReportExportResult> {
  return invoke<ReportExportResult>("report_export", { id });
}

/// 通过固定 Rust command 读取自选股列表。
export async function watchlistList(): Promise<WatchlistList> {
  const response = await invoke<CoreEnvelope<WatchlistList>>("watchlist_list");
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 创建自选股。
export async function watchlistCreate(payload: WatchlistCreatePayload): Promise<WatchlistItem> {
  const response = await invoke<CoreEnvelope<WatchlistItem>>("watchlist_create", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 更新自选股排序、标签和备注。
export async function watchlistUpdate(payload: WatchlistUpdatePayload): Promise<WatchlistItem> {
  const response = await invoke<CoreEnvelope<WatchlistItem>>("watchlist_update", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 删除自选股。
export async function watchlistDelete(id: number): Promise<{ id: number }> {
  const response = await invoke<CoreEnvelope<{ id: number }>>("watchlist_delete", { id });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 提交自选股行情后台刷新请求，不在 UI 线程等待远端行情结果。
export async function watchlistRefresh(payload: WatchlistRefreshPayload): Promise<WatchlistRefreshResult> {
  const response = await invoke<CoreEnvelope<WatchlistRefreshResult>>("watchlist_refresh", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取调度任务列表，前端不接触 Go core 地址或 token。
export async function schedulerJobsList(): Promise<SchedulerJobsList> {
  const response = await invoke<CoreEnvelope<SchedulerJobsList>>(
    "scheduler_jobs_list",
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取单个调度任务。
export async function schedulerJobsGet(id: number): Promise<SchedulerJob> {
  const response = await invoke<CoreEnvelope<SchedulerJob>>(
    "scheduler_jobs_get",
    { id },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取调度任务类型元数据。
export async function schedulerJobTypes(): Promise<SchedulerJobTypes> {
  const response = await invoke<CoreEnvelope<SchedulerJobTypes>>(
    "scheduler_job_types",
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 保存调度任务配置。
export async function schedulerJobsSave(
  payload: SchedulerJobPayload,
): Promise<SchedulerJob> {
  const response = await invoke<CoreEnvelope<SchedulerJob>>(
    "scheduler_jobs_save",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 启用或停用调度任务。
export async function schedulerJobsSetEnabled(
  id: number,
  enabled: boolean,
): Promise<{ id: number; enabled: boolean }> {
  const response = await invoke<CoreEnvelope<{ id: number; enabled: boolean }>>(
    "scheduler_jobs_set_enabled",
    { id, enabled },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 软删除调度任务。
export async function schedulerJobsDelete(
  id: number,
): Promise<{ id: number }> {
  const response = await invoke<CoreEnvelope<{ id: number }>>(
    "scheduler_jobs_delete",
    { id },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 立即执行一次调度任务。
export async function schedulerJobsRunNow(
  payload: SchedulerRunNowPayload,
): Promise<SchedulerRun> {
  const response = await invoke<CoreEnvelope<SchedulerRun>>(
    "scheduler_jobs_run_now",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 触发手动补偿。
export async function schedulerJobsBackfill(
  payload: SchedulerBackfillPayload,
): Promise<SchedulerBackfillResult> {
  const response = await invoke<CoreEnvelope<SchedulerBackfillResult>>(
    "scheduler_jobs_backfill",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取调度执行记录列表。
export async function schedulerRunsList(
  payload: SchedulerRunsListPayload,
): Promise<SchedulerRunsList> {
  const response = await invoke<CoreEnvelope<SchedulerRunsList>>(
    "scheduler_runs_list",
    { jobId: payload.job_id, limit: payload.limit },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取单条调度执行记录。
export async function schedulerRunsGet(id: number): Promise<SchedulerRun> {
  const response = await invoke<CoreEnvelope<SchedulerRun>>(
    "scheduler_runs_get",
    { id },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 手动触发调度执行，复用后端统一执行队列。
export async function schedulerRunsTrigger(
  payload: SchedulerTriggerPayload,
): Promise<SchedulerRun> {
  const response = await invoke<CoreEnvelope<SchedulerRun>>(
    "scheduler_runs_trigger",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取调度摘要状态。
export async function schedulerStatus(): Promise<SchedulerStatus> {
  const response = await invoke<CoreEnvelope<SchedulerStatus>>(
    "scheduler_status",
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 手动刷新某只股票数据，后端会拆成调度 run 并进入统一队列。
export async function schedulerRefreshSymbol(
  payload: SchedulerRefreshSymbolPayload,
): Promise<SchedulerRefreshSymbolResult> {
  const response = await invoke<CoreEnvelope<SchedulerRefreshSymbolResult>>(
    "scheduler_refresh_symbol",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取非敏感设置项。
export async function settingsGet(keys: string[]): Promise<SettingsGetResult> {
  const response = await invoke<CoreEnvelope<SettingsGetResult>>("settings_get", { keys });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 保存设置项，代理密码只作为一次性字段交给 Rust vault。
export async function settingsSet(payload: SettingsSetPayload): Promise<SettingsSetResult> {
  const response = await invoke<CoreEnvelope<SettingsSetResult>>("settings_set", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 执行代理连通性测试，只接受白名单 target 标识。
export async function proxyConnectionTest(payload: ProxyConnectionTestPayload): Promise<ProxyConnectionTestResult> {
  const response = await invoke<CoreEnvelope<ProxyConnectionTestResult>>("proxy_connection_test", { payload });
  return unwrapCoreResponse(response);
}

/// 通过 Rust 白名单命令读取系统开机自启动状态。
export async function autostartGet(): Promise<AutostartState> {
  return invoke<AutostartState>("autostart_get");
}

/// 通过 Rust 白名单命令设置系统开机自启动状态。
export async function autostartSet(enabled: boolean): Promise<AutostartState> {
  return invoke<AutostartState>("autostart_set", { enabled });
}

/// 通过固定 Rust command 读取当前工作区路径。
export async function workspaceGet(): Promise<WorkspaceResult> {
  const response = await invoke<CoreEnvelope<WorkspaceResult>>("workspace_get");
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 保存当前工作区路径。
export async function workspaceSet(path: string): Promise<WorkspaceResult> {
  const response = await invoke<CoreEnvelope<WorkspaceResult>>("workspace_set", { payload: { path } });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 打开当前工作区目录，前端不传任意路径。
export async function workspaceOpen(): Promise<WorkspaceOpenResult> {
  return invoke<WorkspaceOpenResult>("workspace_open");
}

/// 通过固定 Rust command 生成工作区迁移预检计划，不修改本地数据。
export async function workspaceMigrationPlan(targetPath: string): Promise<WorkspaceMigrationPlan> {
  const response = await invoke<CoreEnvelope<WorkspaceMigrationPlan>>("workspace_migration_plan", {
    payload: { target_path: targetPath },
  });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 执行工作区迁移；Rust 会停止并重启 Go core。
export async function workspaceMigrate(payload: WorkspaceMigratePayload): Promise<WorkspaceMigrateResult> {
  const response = await invoke<CoreEnvelope<WorkspaceMigrateResult>>("workspace_migrate", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取可清理缓存统计。
export async function cacheStats(): Promise<CacheStatsResult> {
  const response = await invoke<CoreEnvelope<CacheStatsResult>>("cache_stats");
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 清理允许的临时缓存目标。
export async function cacheClean(targets: string[]): Promise<CacheCleanResult> {
  const response = await invoke<CoreEnvelope<CacheCleanResult>>("cache_clean", { payload: { targets } });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取应用内通知列表。
export async function notificationsList(
  payload: NotificationsListPayload,
): Promise<NotificationsListResult> {
  const response = await invoke<CoreEnvelope<NotificationsListResult>>("notifications_list", {
    payload,
  });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取应用内未读通知数量。
export async function notificationsUnreadCount(): Promise<NotificationsUnreadCountResult> {
  const response = await invoke<CoreEnvelope<NotificationsUnreadCountResult>>(
    "notifications_unread_count",
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 标记指定应用内通知为已读。
export async function notificationsMarkRead(
  payload: NotificationsMarkReadPayload,
): Promise<NotificationsActionResult> {
  const response = await invoke<CoreEnvelope<NotificationsActionResult>>(
    "notifications_mark_read",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 标记全部应用内通知为已读。
export async function notificationsMarkAllRead(): Promise<NotificationsActionResult> {
  const response = await invoke<CoreEnvelope<NotificationsActionResult>>(
    "notifications_mark_all_read",
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 清理已读应用内通知。
export async function notificationsClearRead(): Promise<NotificationsActionResult> {
  const response = await invoke<CoreEnvelope<NotificationsActionResult>>(
    "notifications_clear_read",
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取搜索索引状态，设置中心不直接访问 Go core。
export async function searchStatus(): Promise<SearchIndexStatus> {
  const response = await invoke<CoreEnvelope<SearchIndexStatus>>("search_status");
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 触发搜索索引重建，只允许固定 scope。
export async function searchRebuild(payload: SearchRebuildPayload): Promise<SearchRebuildResult> {
  const response = await invoke<CoreEnvelope<SearchRebuildResult>>("search_rebuild", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 搜索报告历史范围，不接受任意 doc_type。
export async function searchReports(payload: DocumentSearchPayload): Promise<DocumentSearchItem[]> {
  const response = await invoke<CoreEnvelope<DocumentSearchItem[]>>("search_reports", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 搜索资讯中心范围，不接受任意 doc_type。
export async function searchNews(payload: DocumentSearchPayload): Promise<DocumentSearchItem[]> {
  const response = await invoke<CoreEnvelope<DocumentSearchItem[]>>("search_news", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 搜索自选备注范围，不接受任意 doc_type。
export async function searchWatchlistNotes(payload: DocumentSearchPayload): Promise<DocumentSearchItem[]> {
  const response = await invoke<CoreEnvelope<DocumentSearchItem[]>>("search_watchlist_notes", { payload });
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 读取数据源状态。
export async function providersStatus(): Promise<ProvidersStatusResult> {
  const response = await invoke<CoreEnvelope<ProviderStatusItem[] | ProvidersStatusResult>>("providers_status");
  const data = unwrapCoreResponse(response);
  return { items: Array.isArray(data) ? data : data.items };
}

/// 通过固定 Rust command 读取数据源凭据页数据，只接收脱敏展示字段。
export async function dataSourceCredentialsList(): Promise<DataSourceCredentialsListResult> {
  const response = await invoke<CoreEnvelope<DataSourceCredentialsListResult>>(
    "data_source_credentials_list",
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 保存数据源凭据，credential 只作为一次性字段进入 Go core 加密流程。
export async function dataSourceCredentialsSave(
  payload: DataSourceCredentialSavePayload,
): Promise<DataSourceCredentialSaveResult> {
  const response = await invoke<CoreEnvelope<DataSourceCredentialSaveResult>>(
    "data_source_credentials_save",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 清除数据源凭据密文。
export async function dataSourceCredentialsClear(
  payload: DataSourceCredentialClearPayload,
): Promise<DataSourceCredentialClearResult> {
  const response = await invoke<CoreEnvelope<DataSourceCredentialClearResult>>(
    "data_source_credentials_clear",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 执行数据源凭据真实连接测试，只返回脱敏状态和耗时。
export async function dataSourceCredentialsTest(
  payload: DataSourceCredentialTestPayload,
): Promise<DataSourceCredentialTestResponse> {
  const response = await invoke<CoreEnvelope<DataSourceCredentialTestResponse>>(
    "data_source_credentials_test",
    { payload },
  );
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 检查更新提示。
export async function checkUpdate(): Promise<UpdateCheckResult> {
  const response = await invoke<CoreEnvelope<UpdateCheckResult>>("check_update");
  return unwrapCoreResponse(response);
}

/// 通过固定 Rust command 导出已脱敏日志到用户授权目录。
export async function exportLogs(targetDir: string): Promise<ExportLogsResult> {
  return invoke<ExportLogsResult>("export_logs", { targetDir });
}

/// 通过固定 Rust command 打开默认工作区日志目录，初始化失败时不依赖 Go core。
export async function logsOpenDirectory(): Promise<LogsOpenDirectoryResult> {
  return invoke<LogsOpenDirectoryResult>("logs_open_directory");
}

/// 打开系统目录选择器，返回用户授权的本地目录；取消选择时返回 null。
export async function selectDirectory(): Promise<string | null> {
  const selected = await open({ directory: true, multiple: false });
  return typeof selected === "string" ? selected : null;
}
