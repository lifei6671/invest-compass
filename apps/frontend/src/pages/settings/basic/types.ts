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
  taskSuccessNotification: boolean;
  taskFailedNotification: boolean;
};

export type DesktopSettingsState = {
  autostart: boolean;
  closeToTray: boolean;
};

export type CacheSummary = {
  totalSize: string;
  tempSize: string;
  cacheDir: string;
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
  theme: "light",
  language: "zh-CN",
  defaultMarket: "CN",
  defaultAIModel: "DeepSeek-V3",
  quoteRefreshInterval: "60s",
  defaultKlinePeriod: "day",
  defaultAdjustType: "qfq",
};

export const initialWorkspace: WorkspaceSettingsState = {
  workspacePath: "C:\\Users\\InvestCompass\\Documents\\InvestCompass",
};

export const initialNotificationSettings: NotificationSettingsState = {
  taskSuccessNotification: true,
  taskFailedNotification: true,
};

export const initialDesktopSettings: DesktopSettingsState = {
  autostart: false,
  closeToTray: true,
};

export const initialCacheSummary: CacheSummary = {
  totalSize: "512.7 MB",
  tempSize: "128.3 MB",
  cacheDir: "C:\\Users\\InvestCompass\\AppData\\Local\\InvestCompass\\cache",
};

export const initialProxySummary: ProxySummary = {
  mode: "system",
  address: "—",
};

export const initialOtherSettings: OtherSettingsState = {
  checkUpdateOnStartup: true,
  anonymousUsageStats: true,
};
