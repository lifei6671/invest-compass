import { useEffect, useRef } from "react";
import { settingsGet } from "../services/coreClient";
import { settingsKey } from "../pages/settings/basic/settingsKeys";

const MIN_REFRESH_INTERVAL_MS = 5_000;
const DEFAULT_REFRESH_INTERVAL_MS = 60_000;
const refreshIntervalSettingKeys = [settingsKey.quoteRefreshInterval, settingsKey.dataSourceQuoteRefreshInterval] as const;
export const autoRefreshSettingsChangedEvent = "invest-compass:auto-refresh-settings-changed";

export function notifyAutoRefreshSettingsChanged() {
  window.dispatchEvent(new CustomEvent(autoRefreshSettingsChangedEvent));
}

// useAutoRefresh 统一按基础设置中的行情刷新间隔触发页面级刷新，manual 表示关闭自动刷新。
export function useAutoRefresh(onRefresh: () => Promise<void> | void) {
  const refreshRef = useRef(onRefresh);
  const refreshingRef = useRef(false);

  useEffect(() => {
    refreshRef.current = onRefresh;
  }, [onRefresh]);

  useEffect(() => {
    let cancelled = false;
    let installVersion = 0;
    let timer: ReturnType<typeof setInterval> | null = null;

    const clearTimer = () => {
      if (timer) {
        clearInterval(timer);
        timer = null;
      }
    };

    const installTimer = async () => {
      const currentVersion = installVersion + 1;
      installVersion = currentVersion;
      clearTimer();
      try {
        const result = await settingsGet([...refreshIntervalSettingKeys]);
        if (cancelled || currentVersion !== installVersion) {
          return;
        }
        const dataSourceInterval = result.items.find((item) => item.key === settingsKey.dataSourceQuoteRefreshInterval)?.value;
        const basicInterval = result.items.find((item) => item.key === settingsKey.quoteRefreshInterval)?.value;
        const interval = parseRefreshIntervalMs(dataSourceInterval ?? basicInterval);
        if (interval === null) {
          return;
        }
        timer = setInterval(() => {
          if (refreshingRef.current) {
            return;
          }
          refreshingRef.current = true;
          Promise.resolve(refreshRef.current()).finally(() => {
            refreshingRef.current = false;
          });
        }, interval);
      } catch {
        // 设置读取失败时不启动后台定时刷新，避免隐藏真实页面错误来源。
      }
    };

    const handleSettingsChanged = () => {
      void installTimer();
    };

    window.addEventListener(autoRefreshSettingsChangedEvent, handleSettingsChanged);
    void installTimer();

    return () => {
      cancelled = true;
      window.removeEventListener(autoRefreshSettingsChangedEvent, handleSettingsChanged);
      clearTimer();
    };
  }, []);
}

function parseRefreshIntervalMs(value: string | undefined): number | null {
  if (value === "manual") {
    return null;
  }
  const match = /^(\d+)s$/.exec(value ?? "");
  if (!match) {
    return DEFAULT_REFRESH_INTERVAL_MS;
  }
  return Math.max(Number(match[1]) * 1_000, MIN_REFRESH_INTERVAL_MS);
}
