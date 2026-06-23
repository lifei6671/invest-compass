import { defaultSettingsValues } from "./settingsKeys";

export type SettingsTabKey =
  | "basic"
  | "model-config"
  | "prompt-template"
  | "data-source"
  | "proxy"
  | "about";

export type ThemeMode = "light" | "dark" | "system";
export type Language = "zh-CN" | "en-US";
export type Market = "CN" | "HK" | "US";
export type RefreshInterval = "15s" | "30s" | "60s" | "120s" | "manual";
export type KlinePeriod = "minute" | "day" | "week" | "month";
export type AdjustType = "none" | "qfq" | "hfq";

export type BasicSettingsState = {
  theme: ThemeMode;
  language: Language;
  defaultMarket: Market;
  defaultAIModel: string;
  quoteRefreshInterval: RefreshInterval;
  defaultKlinePeriod: KlinePeriod;
  defaultAdjustType: AdjustType;
};

export type WorkspaceSettingsState = {
  workspacePath: string;
};

export type NotificationSettingsState = {
  inAppEnabled: boolean;
  systemEnabled: boolean;
  taskSuccessNotification: boolean;
  taskFailedNotification: boolean;
  providerErrorNotification: boolean;
};

export type DesktopSettingsState = {
  autostart: boolean;
  closeToTray: boolean;
};

export type CacheSummary = {
  totalSize: string;
  tempSize: string;
  cacheDir: string;
  items: Array<{
    target: string;
    label: string;
    size: string;
    cleanable: boolean;
  }>;
};

export type ProxySummary = {
  mode: "system" | "none" | "http" | "socks5";
  address?: string;
};

export type OtherSettingsState = {
  checkUpdateOnStartup: boolean;
  anonymousUsageStats: boolean;
};

export const settingsTabs: Array<{ key: SettingsTabKey; label: string }> = [
  { key: "basic", label: "基础设置" },
  { key: "model-config", label: "模型设置" },
  { key: "prompt-template", label: "Prompt 配置" },
  { key: "data-source", label: "数据源设置" },
  { key: "proxy", label: "代理设置" },
  { key: "about", label: "关于应用" },
];

export const initialBasicSettings: BasicSettingsState = {
  theme: defaultSettingsValues.theme,
  language: defaultSettingsValues.language,
  defaultMarket: defaultSettingsValues.defaultMarket,
  defaultAIModel: "",
  quoteRefreshInterval: defaultSettingsValues.quoteRefreshInterval,
  defaultKlinePeriod: defaultSettingsValues.defaultKlinePeriod,
  defaultAdjustType: defaultSettingsValues.defaultAdjustType,
};

export const initialWorkspace: WorkspaceSettingsState = {
  workspacePath: "",
};

export const initialNotificationSettings: NotificationSettingsState = {
  inAppEnabled: defaultSettingsValues.notificationsInAppEnabled,
  systemEnabled: defaultSettingsValues.notificationsSystemEnabled,
  taskSuccessNotification: defaultSettingsValues.taskSuccessNotification,
  taskFailedNotification: defaultSettingsValues.taskFailedNotification,
  providerErrorNotification: defaultSettingsValues.providerErrorNotification,
};

export const initialDesktopSettings: DesktopSettingsState = {
  autostart: false,
  closeToTray: defaultSettingsValues.closeToTray,
};

export const initialCacheSummary: CacheSummary = {
  totalSize: "—",
  tempSize: "—",
  cacheDir: "由本地核心服务管理",
  items: [],
};

export const initialProxySummary: ProxySummary = {
  mode: "system",
  address: "—",
};

export const initialOtherSettings: OtherSettingsState = {
  checkUpdateOnStartup: defaultSettingsValues.checkUpdateOnStartup,
  anonymousUsageStats: true,
};
