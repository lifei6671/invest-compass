import { App as AntApp, Checkbox, Modal } from "antd";
import { useCallback, useEffect, useState } from "react";
import {
  aiConfigList,
  aiConfigSave,
  autostartGet,
  autostartSet,
  cacheClean,
  cacheStats,
  searchRebuild,
  searchStatus,
  selectDirectory,
  settingsGet,
  settingsSet,
  workspaceOpen,
  workspaceMigrate,
  workspaceMigrationPlan,
  workspaceGet,
  type AIConfig,
  type CacheStatsItem,
  type CacheStatsResult,
  type SettingItem,
  type SearchIndexStatus,
  type SearchRebuildPayload,
  type WorkspaceMigrationPlan,
} from "../../../services/coreClient";
import { AppBasicSettingsCard } from "./components/AppBasicSettingsCard";
import { CacheManagementCard } from "./components/CacheManagementCard";
import { DesktopCapabilityCard } from "./components/DesktopCapabilityCard";
import { NotificationSettingsCard } from "./components/NotificationSettingsCard";
import { ProxySummaryCard } from "./components/ProxySummaryCard";
import { SearchIndexManagementCard } from "./components/SearchIndexManagementCard";
import { WorkspaceSettingsCard } from "./components/WorkspaceSettingsCard";
import { SettingsRiskNotice } from "../components/SettingsRiskNotice";
import { useDashboardStore } from "../../../stores/dashboardStore";
import {
  basicSettingsKeys,
  notificationSettingsKeys,
  settingsKey,
} from "./settingsKeys";
import {
  initialBasicSettings,
  initialCacheSummary,
  initialDesktopSettings,
  initialNotificationSettings,
  initialProxySummary,
  initialWorkspace,
  type BasicSettingsState,
  type CacheSummary,
  type DesktopSettingsState,
  type NotificationSettingsState,
  type WorkspaceSettingsState,
} from "./types";

type PersistedBasicSettingsKey = Exclude<keyof BasicSettingsState, "defaultAIModel">;
type InitialLoadOptions = {
  initial?: boolean;
};

const settingsInitialLoadErrorKey = "settings-basic-initial-load-error";
const searchIndexInitialRetryDelayMs = 200;
const cacheTargetLabels: Record<string, string> = {
  quote: "行情缓存",
  kline: "K 线缓存",
  news: "资讯缓存",
  chart_image: "图表缓存",
  task_logs: "任务日志",
  app_logs: "应用日志",
};

const basicSettingKeyByField: Record<PersistedBasicSettingsKey, string> = {
  theme: settingsKey.appTheme,
  language: settingsKey.appLanguage,
  defaultMarket: settingsKey.marketDefault,
  quoteRefreshInterval: settingsKey.quoteRefreshInterval,
  defaultKlinePeriod: settingsKey.klineDefaultPeriod,
  defaultAdjustType: settingsKey.klineDefaultAdjust,
};

const notificationSettingKeyByField: Record<keyof NotificationSettingsState, string> = {
  inAppEnabled: settingsKey.notificationsInAppEnabled,
  systemEnabled: settingsKey.notificationsSystemEnabled,
  taskSuccessNotification: settingsKey.notificationsTaskSuccess,
  taskFailedNotification: settingsKey.notificationsTaskFailed,
  providerErrorNotification: settingsKey.notificationsProviderError,
};

export function SettingsBasicPage() {
  const { message } = AntApp.useApp();
  const [basicSettings, setBasicSettings] = useState<BasicSettingsState>(initialBasicSettings);
  const [aiConfigs, setAIConfigs] = useState<AIConfig[]>([]);
  const [workspaceSettings, setWorkspaceSettings] = useState<WorkspaceSettingsState>(initialWorkspace);
  const [notificationSettings, setNotificationSettings] = useState<NotificationSettingsState>(initialNotificationSettings);
  const [desktopSettings, setDesktopSettings] = useState<DesktopSettingsState>(initialDesktopSettings);
  const [cacheSummary, setCacheSummary] = useState<CacheSummary>(initialCacheSummary);
  const [cacheLoading, setCacheLoading] = useState(false);
  const [cacheConfirmOpen, setCacheConfirmOpen] = useState(false);
  const [searchIndexStatus, setSearchIndexStatus] = useState<SearchIndexStatus | null>(null);
  const [searchIndexLoading, setSearchIndexLoading] = useState(false);
  const [rebuildingScope, setRebuildingScope] = useState<SearchRebuildPayload["scope"] | null>(null);
  const [migrationPlan, setMigrationPlan] = useState<WorkspaceMigrationPlan | null>(null);
  const [migrationLoading, setMigrationLoading] = useState(false);
  const [migrationCreateBackup, setMigrationCreateBackup] = useState(true);
  const [migrationIncludeCache, setMigrationIncludeCache] = useState(false);

  const reportCoreError = useCallback((error: unknown) => {
    const messageText = safeErrorMessage(error, "");
    if (!isCoreUnavailableMessage(messageText)) {
      return;
    }
    useDashboardStore.setState({
      state: null,
      error: messageText || "本地核心服务连接失败",
    });
  }, []);

  const showInitialLoadError = useCallback((source: string) => {
    message.error({ key: settingsInitialLoadErrorKey, content: `${source}读取失败，请稍后重试` });
  }, [message]);

  const handleInitialLoadError = useCallback((source: string, error: unknown) => {
    reportCoreError(error);
    showInitialLoadError(source);
  }, [reportCoreError, showInitialLoadError]);

  const loadBasicSettings = useCallback(async () => {
    try {
      const [settings, configs] = await Promise.all([settingsGet([...basicSettingsKeys]), aiConfigList()]);
      const nextAIConfigs = configs.items;
      setAIConfigs(nextAIConfigs);
      setBasicSettings((current) => ({
        ...current,
        ...basicSettingsFromItems(settings.items),
        defaultAIModel: String(nextAIConfigs.find((item) => item.is_default)?.id ?? ""),
      }));
    } catch (error) {
      handleInitialLoadError("基础设置", error);
    }
  }, [handleInitialLoadError]);

  useEffect(() => {
    void loadBasicSettings();
  }, [loadBasicSettings]);

  const loadWorkspaceSettings = useCallback(async () => {
    try {
      const workspace = await workspaceGet();
      setWorkspaceSettings({ workspacePath: workspace.path });
    } catch (error) {
      handleInitialLoadError("工作区设置", error);
    }
  }, [handleInitialLoadError]);

  useEffect(() => {
    void loadWorkspaceSettings();
  }, [loadWorkspaceSettings]);

  const loadNotificationSettings = useCallback(async () => {
    try {
      const settings = await settingsGet([...notificationSettingsKeys]);
      setNotificationSettings((current) => ({
        ...current,
        ...notificationSettingsFromItems(settings.items),
      }));
    } catch (error) {
      handleInitialLoadError("通知设置", error);
    }
  }, [handleInitialLoadError]);

  useEffect(() => {
    void loadNotificationSettings();
  }, [loadNotificationSettings]);

  const loadCacheSummary = useCallback(async (options: InitialLoadOptions = {}) => {
    try {
      const stats = await cacheStats();
      setCacheSummary(buildCacheSummary(stats));
    } catch (error) {
      if (options.initial) {
        handleInitialLoadError("缓存统计", error);
      } else {
        reportCoreError(error);
        message.error(error instanceof Error ? error.message : "缓存统计读取失败");
      }
      setCacheSummary(initialCacheSummary);
    }
  }, [handleInitialLoadError, message, reportCoreError]);

  useEffect(() => {
    void loadCacheSummary({ initial: true });
  }, [loadCacheSummary]);

  const loadSearchIndexStatus = useCallback(async (options: InitialLoadOptions = {}) => {
    setSearchIndexLoading(true);
    try {
      setSearchIndexStatus(await readSearchIndexStatusWithInitialRetry(options));
    } catch (error) {
      if (options.initial) {
        handleInitialLoadError("搜索索引状态", error);
      } else {
        reportCoreError(error);
        message.error(error instanceof Error ? error.message : "搜索索引状态读取失败");
      }
      setSearchIndexStatus(null);
    } finally {
      setSearchIndexLoading(false);
    }
  }, [handleInitialLoadError, message, reportCoreError]);

  useEffect(() => {
    void loadSearchIndexStatus({ initial: true });
  }, [loadSearchIndexStatus]);

  const loadDesktopSettings = useCallback(async () => {
    try {
      const [autostart, settings] = await Promise.all([
        autostartGet(),
        settingsGet([settingsKey.windowCloseToTray]),
      ]);
      setDesktopSettings((current) => ({
        ...current,
        autostart: autostart.enabled,
        closeToTray: readBooleanSetting(
          settings.items,
          settingsKey.windowCloseToTray,
          initialDesktopSettings.closeToTray,
        ),
      }));
    } catch (error) {
      handleInitialLoadError("桌面设置", error);
    }
  }, [handleInitialLoadError]);

  useEffect(() => {
    void loadDesktopSettings();
  }, [loadDesktopSettings]);

  const handleRequestCleanCache = () => {
    const targets = cacheSummary.items.filter((item) => item.cleanable).map((item) => item.target);
    if (targets.length === 0) {
      message.info("暂无可清理缓存");
      return;
    }
    setCacheConfirmOpen(true);
  };

  const handleCleanCache = async () => {
    const targets = cacheSummary.items.filter((item) => item.cleanable).map((item) => item.target);
    if (targets.length === 0) {
      message.info("暂无可清理缓存");
      return;
    }
    setCacheLoading(true);
    try {
      await cacheClean(targets);
      await loadCacheSummary();
      setCacheConfirmOpen(false);
      message.success("缓存清理完成");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "缓存清理失败");
    } finally {
      setCacheLoading(false);
    }
  };

  const handleRebuildSearchIndex = async (scope: SearchRebuildPayload["scope"]) => {
    setRebuildingScope(scope);
    try {
      const result = await searchRebuild({ scope });
      await loadSearchIndexStatus();
      message.success(`索引重建任务已创建：${result.task_id}，可在任务历史查看`);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "索引重建失败");
    } finally {
      setRebuildingScope(null);
    }
  };

  const handleBasicSettingsChange = (nextValue: BasicSettingsState) => {
    const previousValue = basicSettings;
    setBasicSettings(nextValue);

    const changedKey = findChangedBasicSettingsKey(previousValue, nextValue);
    if (!changedKey) {
      return;
    }
    if (changedKey === "defaultAIModel") {
      void saveDefaultAIModel(nextValue.defaultAIModel, previousValue);
      return;
    }
    void saveBasicSetting(changedKey, nextValue[changedKey], previousValue);
  };

  const saveBasicSetting = async (field: PersistedBasicSettingsKey, value: string, previousValue: BasicSettingsState) => {
    try {
      await settingsSet({ items: [{ key: basicSettingKeyByField[field], value }] });
      message.success("设置已更新");
    } catch (error) {
      setBasicSettings(previousValue);
      message.error(error instanceof Error ? error.message : "设置保存失败");
    }
  };

  const saveDefaultAIModel = async (configId: string, previousValue: BasicSettingsState) => {
    const selectedConfig = aiConfigs.find((item) => String(item.id) === configId);
    if (!selectedConfig) {
      setBasicSettings(previousValue);
      message.warning("请选择已配置的 AI 模型");
      return;
    }
    try {
      await aiConfigSave({ ...selectedConfig, is_default: true });
      const configs = await aiConfigList();
      setAIConfigs(configs.items);
      setBasicSettings((current) => ({
        ...current,
        defaultAIModel: String(configs.items.find((item) => item.is_default)?.id ?? selectedConfig.id),
      }));
      message.success("默认模型已更新");
    } catch (error) {
      setBasicSettings(previousValue);
      message.error(error instanceof Error ? error.message : "默认模型保存失败");
    }
  };

  const handleSelectWorkspaceDirectory = async () => {
    try {
      const selectedPath = await selectDirectory();
      if (!selectedPath) {
        return;
      }
      const plan = await workspaceMigrationPlan(selectedPath);
      if (!plan.can_migrate) {
        message.error(plan.reason || "工作区迁移预检未通过");
      }
      setMigrationCreateBackup(true);
      setMigrationIncludeCache(false);
      setMigrationPlan(plan);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "工作区迁移预检失败");
    }
  };

  const handleConfirmWorkspaceMigration = async () => {
    if (!migrationPlan || !migrationPlan.can_migrate) {
      return;
    }
    setMigrationLoading(true);
    try {
      const result = await workspaceMigrate({
        target_path: migrationPlan.target_path,
        include_cache: migrationIncludeCache,
        create_backup: migrationCreateBackup,
      });
      setWorkspaceSettings({ workspacePath: result.path });
      setMigrationPlan(null);
      message.success("工作区迁移完成");
    } catch (error) {
      message.error(error instanceof Error ? error.message : "工作区迁移失败");
    } finally {
      setMigrationLoading(false);
    }
  };

  const handleOpenWorkspaceDirectory = async () => {
    try {
      await workspaceOpen();
    } catch (error) {
      message.error(error instanceof Error ? error.message : "打开工作区目录失败");
    }
  };

  const handleDesktopSettingsChange = (nextValue: DesktopSettingsState) => {
    const previousValue = desktopSettings;
    setDesktopSettings(nextValue);
    if (previousValue.autostart !== nextValue.autostart) {
      void saveAutostartSetting(nextValue.autostart, previousValue);
      return;
    }
    if (previousValue.closeToTray !== nextValue.closeToTray) {
      void saveCloseToTraySetting(nextValue.closeToTray, previousValue);
    }
  };

  const handleNotificationSettingsChange = (nextValue: NotificationSettingsState) => {
    const previousValue = notificationSettings;
    setNotificationSettings(nextValue);
    const changedKey = findChangedNotificationSettingsKey(previousValue, nextValue);
    if (!changedKey) {
      return;
    }
    void saveNotificationSetting(changedKey, nextValue[changedKey], previousValue);
  };

  const saveNotificationSetting = async (field: keyof NotificationSettingsState, value: boolean, previousValue: NotificationSettingsState) => {
    try {
      await settingsSet({
        items: [{ key: notificationSettingKeyByField[field], value: String(value) }],
      });
      message.success("通知设置已更新");
    } catch (error) {
      setNotificationSettings(previousValue);
      message.error(error instanceof Error ? error.message : "通知设置保存失败");
    }
  };

  const saveAutostartSetting = async (enabled: boolean, previousValue: DesktopSettingsState) => {
    try {
      const result = await autostartSet(enabled);
      setDesktopSettings((current) => ({ ...current, autostart: result.enabled }));
      message.success("桌面能力设置已更新");
    } catch (error) {
      setDesktopSettings(previousValue);
      message.error(error instanceof Error ? error.message : "开机自启动设置失败");
    }
  };

  const saveCloseToTraySetting = async (enabled: boolean, previousValue: DesktopSettingsState) => {
    try {
      await settingsSet({
        items: [{ key: settingsKey.windowCloseToTray, value: String(enabled) }],
      });
      message.success("桌面能力设置已更新");
    } catch (error) {
      setDesktopSettings(previousValue);
      message.error(error instanceof Error ? error.message : "关闭到托盘设置失败");
    }
  };

  return (
    <>
      <AppBasicSettingsCard
        value={basicSettings}
        aiModelOptions={aiConfigs.map((config) => ({
          label: `${config.name} / ${config.model_name}`,
          value: String(config.id),
        }))}
        onChange={handleBasicSettingsChange}
      />
      <div className="settings-basic-card-grid">
        <WorkspaceSettingsCard
          value={workspaceSettings}
          onSelectDirectory={handleSelectWorkspaceDirectory}
          onOpenDirectory={handleOpenWorkspaceDirectory}
        />
        <NotificationSettingsCard
          value={notificationSettings}
          onChange={handleNotificationSettingsChange}
        />
        <DesktopCapabilityCard
          value={desktopSettings}
          onChange={handleDesktopSettingsChange}
        />
        <CacheManagementCard
          value={cacheSummary}
          loading={cacheLoading}
          onCleanCache={handleRequestCleanCache}
        />
        <SearchIndexManagementCard
          value={searchIndexStatus}
          loading={searchIndexLoading}
          rebuildingScope={rebuildingScope}
          onRefresh={loadSearchIndexStatus}
          onRebuild={handleRebuildSearchIndex}
        />
        <ProxySummaryCard value={initialProxySummary} onEditProxy={() => message.info("代理设置页待接入")} />
      </div>
      <Modal
        title="确认清理缓存"
        open={cacheConfirmOpen}
        okText="确认清理"
        cancelText="取消"
        confirmLoading={cacheLoading}
        onOk={handleCleanCache}
        onCancel={() => setCacheConfirmOpen(false)}
      >
        <div className="settings-basic-cache-confirm">
          <p>将清理行情、K 线、资讯、图表和任务日志等临时缓存。</p>
          <p>报告、配置和凭据不会被删除。</p>
          <strong>预计可清理：{cacheSummary.tempSize}</strong>
        </div>
      </Modal>
      <Modal
        title="确认迁移工作区"
        open={migrationPlan !== null}
        okText="确认迁移"
        cancelText="取消"
        confirmLoading={migrationLoading}
        okButtonProps={{ disabled: migrationPlan ? !migrationPlan.can_migrate : false }}
        onOk={handleConfirmWorkspaceMigration}
        onCancel={() => setMigrationPlan(null)}
      >
        {migrationPlan ? (
          <div className="settings-basic-migration-modal">
            <div className={`settings-basic-migration-status ${migrationPlan.can_migrate ? "is-success" : "is-error"}`}>
              {migrationPlan.can_migrate ? "预检通过，可以迁移工作区。" : migrationPlan.reason || "工作区迁移预检未通过"}
            </div>
            <div className="settings-basic-migration-row">
              <span>当前工作区</span>
              <strong>{migrationPlan.current_path}</strong>
            </div>
            <div className="settings-basic-migration-row">
              <span>目标工作区</span>
              <strong>{migrationPlan.target_path}</strong>
            </div>
            <div className="settings-basic-migration-row">
              <span>迁移数据量</span>
              <strong>{formatBytes(migrationPlan.source_size_bytes)}</strong>
            </div>
            <div className="settings-basic-migration-row">
              <span>可用空间</span>
              <strong>{formatOptionalBytes(migrationPlan.available_space_bytes)}</strong>
            </div>
            <Checkbox checked={migrationCreateBackup} onChange={(event) => setMigrationCreateBackup(event.target.checked)}>
              创建迁移前备份
            </Checkbox>
            <Checkbox checked={migrationIncludeCache} onChange={(event) => setMigrationIncludeCache(event.target.checked)}>
              包含缓存目录
            </Checkbox>
            {migrationPlan.warnings.length > 0 ? (
              <ul className="settings-basic-migration-warnings">
                {migrationPlan.warnings.map((warning) => (
                  <li key={warning}>{warning}</li>
                ))}
              </ul>
            ) : null}
          </div>
        ) : null}
      </Modal>
      <SettingsRiskNotice />
    </>
  );
}

function basicSettingsFromItems(items: SettingItem[]): Partial<BasicSettingsState> {
  const values = new Map(items.map((item) => [item.key, item.value]));
  const settings: Partial<BasicSettingsState> = {};
  assignIfDefined(settings, "theme", readEnum(values, settingsKey.appTheme, ["light", "dark", "system"]));
  assignIfDefined(settings, "language", readEnum(values, settingsKey.appLanguage, ["zh-CN", "en-US"]));
  assignIfDefined(settings, "defaultMarket", readEnum(values, settingsKey.marketDefault, ["CN", "HK", "US"]));
  assignIfDefined(settings, "quoteRefreshInterval", readEnum(values, settingsKey.quoteRefreshInterval, ["15s", "30s", "60s", "120s", "manual"]));
  assignIfDefined(settings, "defaultKlinePeriod", readEnum(values, settingsKey.klineDefaultPeriod, ["minute", "day", "week", "month"]));
  assignIfDefined(settings, "defaultAdjustType", readEnum(values, settingsKey.klineDefaultAdjust, ["none", "qfq", "hfq"]));
  return settings;
}

function notificationSettingsFromItems(items: SettingItem[]): Partial<NotificationSettingsState> {
  return {
    inAppEnabled: readBooleanSetting(items, settingsKey.notificationsInAppEnabled, initialNotificationSettings.inAppEnabled),
    systemEnabled: readBooleanSetting(items, settingsKey.notificationsSystemEnabled, initialNotificationSettings.systemEnabled),
    taskSuccessNotification: readBooleanSetting(items, settingsKey.notificationsTaskSuccess, initialNotificationSettings.taskSuccessNotification),
    taskFailedNotification: readBooleanSetting(items, settingsKey.notificationsTaskFailed, initialNotificationSettings.taskFailedNotification),
    providerErrorNotification: readBooleanSetting(items, settingsKey.notificationsProviderError, initialNotificationSettings.providerErrorNotification),
  };
}

function readBooleanSetting(items: SettingItem[], key: string, fallback: boolean): boolean {
  const value = items.find((item) => item.key === key)?.value;
  if (value === "true") {
    return true;
  }
  if (value === "false") {
    return false;
  }
  return fallback;
}

function readEnum<Value extends string>(values: Map<string, string>, key: string, allowedValues: Value[]): Value | undefined {
  const value = values.get(key);
  return allowedValues.includes(value as Value) ? (value as Value) : undefined;
}

function assignIfDefined<Key extends keyof BasicSettingsState>(settings: Partial<BasicSettingsState>, key: Key, value: BasicSettingsState[Key] | undefined) {
  if (value !== undefined) {
    settings[key] = value;
  }
}

function findChangedBasicSettingsKey(previousValue: BasicSettingsState, nextValue: BasicSettingsState): keyof BasicSettingsState | null {
  for (const key of Object.keys(nextValue) as Array<keyof BasicSettingsState>) {
    if (previousValue[key] !== nextValue[key]) {
      return key;
    }
  }
  return null;
}

function findChangedNotificationSettingsKey(previousValue: NotificationSettingsState, nextValue: NotificationSettingsState): keyof NotificationSettingsState | null {
  for (const key of Object.keys(nextValue) as Array<keyof NotificationSettingsState>) {
    if (previousValue[key] !== nextValue[key]) {
      return key;
    }
  }
  return null;
}

function buildCacheSummary(stats: CacheStatsResult): CacheSummary {
  const items = stats.items.map(normalizeCacheStatsItem);
  const cleanableBytes = stats.items.reduce((sum, item) => sum + (item.cleanable === false ? 0 : Math.max(item.bytes, 0)), 0);
  return {
    totalSize: formatBytes(stats.total_bytes),
    tempSize: formatBytes(cleanableBytes),
    cacheDir: "由本地核心服务管理",
    items,
  };
}

function normalizeCacheStatsItem(item: CacheStatsItem): CacheSummary["items"][number] {
  return {
    target: item.target,
    label: item.label || cacheTargetLabels[item.target] || item.target,
    size: formatBytes(item.bytes),
    cleanable: item.cleanable !== false,
  };
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "0 MB";
  }
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  const fractionDigits = unitIndex === 0 ? 0 : 1;
  return `${value.toFixed(fractionDigits)} ${units[unitIndex]}`;
}

function formatOptionalBytes(bytes: number | null): string {
  return bytes === null ? "无法读取" : formatBytes(bytes);
}

function safeErrorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : typeof error === "string" ? error : fallback;
}

function isCoreUnavailableMessage(value: string): boolean {
  return value.includes("core sidecar is not running") || value.includes("sidecar http error");
}

async function readSearchIndexStatusWithInitialRetry(options: InitialLoadOptions): Promise<SearchIndexStatus> {
  try {
    return await searchStatus();
  } catch (error) {
    if (!options.initial || !isCoreUnavailableMessage(safeErrorMessage(error, ""))) {
      throw error;
    }
    await delay(searchIndexInitialRetryDelayMs);
    return searchStatus();
  }
}

function delay(milliseconds: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, milliseconds);
  });
}
