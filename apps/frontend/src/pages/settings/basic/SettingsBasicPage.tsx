import { App as AntApp } from "antd";
import { useCallback, useEffect, useState } from "react";
import {
  cacheClean,
  cacheStats,
  searchRebuild,
  searchStatus,
  type CacheStatsItem,
  type CacheStatsResult,
  type SearchIndexStatus,
  type SearchRebuildPayload,
} from "../../../services/coreClient";
import { AppBasicSettingsCard } from "./components/AppBasicSettingsCard";
import { CacheManagementCard } from "./components/CacheManagementCard";
import { DesktopCapabilityCard } from "./components/DesktopCapabilityCard";
import { NotificationSettingsCard } from "./components/NotificationSettingsCard";
import { OtherSettingsCard } from "./components/OtherSettingsCard";
import { ProxySummaryCard } from "./components/ProxySummaryCard";
import { SearchIndexManagementCard } from "./components/SearchIndexManagementCard";
import { WorkspaceSettingsCard } from "./components/WorkspaceSettingsCard";
import { SettingsRiskNotice } from "../components/SettingsRiskNotice";
import {
  initialBasicSettings,
  initialCacheSummary,
  initialDesktopSettings,
  initialNotificationSettings,
  initialOtherSettings,
  initialProxySummary,
  initialWorkspace,
  type BasicSettingsState,
  type CacheSummary,
  type DesktopSettingsState,
  type NotificationSettingsState,
  type OtherSettingsState,
} from "./types";

const cacheTargetLabels: Record<string, string> = {
  quote: "行情缓存",
  kline: "K 线缓存",
  news: "资讯缓存",
  chart_image: "图表缓存",
  task_logs: "任务日志",
  app_logs: "应用日志",
};

export function SettingsBasicPage() {
  const { message } = AntApp.useApp();
  const [basicSettings, setBasicSettings] = useState<BasicSettingsState>(initialBasicSettings);
  const [notificationSettings, setNotificationSettings] = useState<NotificationSettingsState>(initialNotificationSettings);
  const [desktopSettings, setDesktopSettings] = useState<DesktopSettingsState>(initialDesktopSettings);
  const [cacheSummary, setCacheSummary] = useState<CacheSummary>(initialCacheSummary);
  const [cacheLoading, setCacheLoading] = useState(false);
  const [searchIndexStatus, setSearchIndexStatus] = useState<SearchIndexStatus | null>(null);
  const [searchIndexLoading, setSearchIndexLoading] = useState(false);
  const [rebuildingScope, setRebuildingScope] = useState<SearchRebuildPayload["scope"] | null>(null);
  const [otherSettings, setOtherSettings] = useState<OtherSettingsState>(initialOtherSettings);

  const loadCacheSummary = useCallback(async () => {
    try {
      const stats = await cacheStats();
      setCacheSummary(buildCacheSummary(stats));
    } catch (error) {
      message.error(error instanceof Error ? error.message : "缓存统计读取失败");
      setCacheSummary(initialCacheSummary);
    }
  }, [message]);

  useEffect(() => {
    void loadCacheSummary();
  }, [loadCacheSummary]);

  const loadSearchIndexStatus = useCallback(async () => {
    setSearchIndexLoading(true);
    try {
      setSearchIndexStatus(await searchStatus());
    } catch (error) {
      message.error(error instanceof Error ? error.message : "搜索索引状态读取失败");
      setSearchIndexStatus(null);
    } finally {
      setSearchIndexLoading(false);
    }
  }, [message]);

  useEffect(() => {
    void loadSearchIndexStatus();
  }, [loadSearchIndexStatus]);

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
      const result = await searchRebuild({ scope, force: false });
      await loadSearchIndexStatus();
      message.success(`索引重建任务已创建：${result.task_id}`);
    } catch (error) {
      message.error(error instanceof Error ? error.message : "索引重建失败");
    } finally {
      setRebuildingScope(null);
    }
  };

  return (
    <>
      <AppBasicSettingsCard
        value={basicSettings}
        onChange={(value) => {
          setBasicSettings(value);
          message.success("设置已更新");
        }}
      />
      <div className="settings-basic-card-grid">
        <WorkspaceSettingsCard
          value={initialWorkspace}
          onSelectDirectory={() => message.info("选择目录待接入")}
          onOpenDirectory={() => message.info("打开目录待接入")}
        />
        <NotificationSettingsCard
          value={notificationSettings}
          onChange={(value) => {
            setNotificationSettings(value);
            message.success("通知设置已更新");
          }}
        />
        <DesktopCapabilityCard
          value={desktopSettings}
          onChange={(value) => {
            setDesktopSettings(value);
            message.success("桌面能力设置已更新");
          }}
        />
        <CacheManagementCard
          value={cacheSummary}
          loading={cacheLoading}
          onCleanCache={handleCleanCache}
        />
        <SearchIndexManagementCard
          value={searchIndexStatus}
          loading={searchIndexLoading}
          rebuildingScope={rebuildingScope}
          onRefresh={loadSearchIndexStatus}
          onRebuild={handleRebuildSearchIndex}
        />
        <ProxySummaryCard value={initialProxySummary} onEditProxy={() => message.info("代理设置页待接入")} />
        <OtherSettingsCard
          value={otherSettings}
          onChange={(value) => {
            setOtherSettings(value);
            message.success("设置已更新");
          }}
        />
      </div>
      <SettingsRiskNotice />
    </>
  );
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
