import type { AdjustType, KlinePeriod, Language, Market, RefreshInterval, ThemeMode } from "./types";

export const settingsKey = {
  appTheme: "app.theme",
  appLanguage: "app.language",
  marketDefault: "market.default",
  quoteRefreshInterval: "quote.refresh_interval",
  dataSourceQuoteRefreshInterval: "data_source.quote_refresh_interval",
  klineDefaultPeriod: "kline.default_period",
  klineDefaultAdjust: "kline.default_adjust",
  notificationsInAppEnabled: "notifications.in_app_enabled",
  notificationsSystemEnabled: "notifications.system_enabled",
  notificationsTaskSuccess: "notifications.task_success",
  notificationsTaskFailed: "notifications.task_failed",
  notificationsProviderError: "notifications.provider_error",
  windowCloseToTray: "window.close_to_tray",
  updateCheckOnStartup: "update.check_on_startup",
} as const;

export const basicSettingsKeys = [
  settingsKey.appTheme,
  settingsKey.appLanguage,
  settingsKey.marketDefault,
  settingsKey.quoteRefreshInterval,
  settingsKey.klineDefaultPeriod,
  settingsKey.klineDefaultAdjust,
] as const;

export const notificationSettingsKeys = [
  settingsKey.notificationsInAppEnabled,
  settingsKey.notificationsSystemEnabled,
  settingsKey.notificationsTaskSuccess,
  settingsKey.notificationsTaskFailed,
  settingsKey.notificationsProviderError,
] as const;

export const defaultSettingsValues = {
  theme: "light" satisfies ThemeMode,
  language: "zh-CN" satisfies Language,
  defaultMarket: "CN" satisfies Market,
  quoteRefreshInterval: "60s" satisfies RefreshInterval,
  defaultKlinePeriod: "day" satisfies KlinePeriod,
  defaultAdjustType: "qfq" satisfies AdjustType,
  notificationsInAppEnabled: true,
  notificationsSystemEnabled: true,
  taskSuccessNotification: true,
  taskFailedNotification: true,
  providerErrorNotification: true,
  closeToTray: true,
  checkUpdateOnStartup: true,
} as const;
