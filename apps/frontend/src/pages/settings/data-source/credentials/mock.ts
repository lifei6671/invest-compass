import type {
  CredentialConfig,
  CredentialHealthItem,
  CredentialOperationLog,
  CredentialOverview,
  CredentialTestResult,
  CredentialTestTarget,
  DataSourceProvider,
} from "./types";

export const credentialProviders: DataSourceProvider[] = [
  { id: "eastmoney", name: "EastMoney", capability: "行情 / K线", status: "normal", authType: "none", iconType: "eastmoney" },
  { id: "akshare", name: "AkShare", capability: "基础数据", status: "normal", authType: "none", iconType: "akshare" },
  { id: "alpha-vantage", name: "Alpha Vantage", capability: "海外行情", status: "not_configured", authType: "api_key", iconType: "alpha" },
  { id: "cls", name: "财联社", capability: "快讯 / 日历", status: "normal", authType: "cookie", iconType: "cls" },
  { id: "xueqiu", name: "雪球", capability: "讨论热度", status: "not_configured", authType: "cookie", iconType: "xueqiu" },
  { id: "custom-http", name: "Custom HTTP", capability: "自定义接口", status: "not_configured", authType: "bearer_token", iconType: "custom" },
];

export const selectedProviderId = "cls";

export const credentialConfigs: Record<string, CredentialConfig> = {
  eastmoney: {
    providerId: "eastmoney",
    providerName: "EastMoney",
    capability: "行情 / K线",
    authType: "none",
    baseUrl: "https://quote.eastmoney.com",
    credentialStatus: "normal",
    expiresAt: "",
    timeoutSeconds: 15,
    rateLimitPerMinute: 60,
    maskedCredential: "无需凭据",
    note: "",
  },
  akshare: {
    providerId: "akshare",
    providerName: "AkShare",
    capability: "基础数据",
    authType: "none",
    baseUrl: "https://akshare.akfamily.xyz",
    credentialStatus: "normal",
    expiresAt: "",
    timeoutSeconds: 20,
    rateLimitPerMinute: 45,
    maskedCredential: "无需凭据",
    note: "",
  },
  "alpha-vantage": {
    providerId: "alpha-vantage",
    providerName: "Alpha Vantage",
    capability: "海外行情",
    authType: "api_key",
    baseUrl: "https://www.alphavantage.co",
    credentialStatus: "not_configured",
    expiresAt: "",
    timeoutSeconds: 15,
    rateLimitPerMinute: 12,
    maskedCredential: "key=****",
    note: "",
  },
  cls: {
    providerId: "cls",
    providerName: "财联社",
    capability: "快讯 / 行业事件 / 日历",
    authType: "cookie",
    baseUrl: "https://www.cls.cn",
    credentialStatus: "normal",
    expiresAt: "2025-06-30 23:59",
    timeoutSeconds: 15,
    rateLimitPerMinute: 30,
    maskedCredential: "uid=****; token=****; session=****",
    note: "",
  },
  xueqiu: {
    providerId: "xueqiu",
    providerName: "雪球",
    capability: "讨论热度",
    authType: "cookie",
    baseUrl: "https://xueqiu.com",
    credentialStatus: "not_configured",
    expiresAt: "",
    timeoutSeconds: 15,
    rateLimitPerMinute: 20,
    maskedCredential: "xq_a_token=****",
    note: "",
  },
  "custom-http": {
    providerId: "custom-http",
    providerName: "Custom HTTP",
    capability: "自定义接口",
    authType: "bearer_token",
    baseUrl: "https://api.example.com",
    credentialStatus: "not_configured",
    expiresAt: "",
    timeoutSeconds: 15,
    rateLimitPerMinute: 30,
    maskedCredential: "Bearer ****",
    note: "",
  },
};

export const credentialTestTargets: CredentialTestTarget[] = [
  { label: "快讯接口（/api/flash）", value: "flash" },
  { label: "日历接口（/api/calendar）", value: "calendar" },
  { label: "行业事件接口（/api/events）", value: "events" },
];

export const credentialTestResult: CredentialTestResult = {
  status: "success",
  responseTimeMs: 186,
  testedAt: "2025-05-20 15:28:41",
  messages: ["行情接口可访问", "新闻接口已授权"],
};

export const credentialOverview: CredentialOverview = {
  configuredCount: 3,
  expiringSoonCount: 1,
  expiredCount: 1,
};

export const credentialHealthItems: CredentialHealthItem[] = [
  { name: "行情源", status: "normal", rateLimitText: "28 次/分钟" },
  { name: "新闻源", status: "normal", rateLimitText: "26 次/分钟" },
  { name: "海外源", status: "limited", rateLimitText: "12 次/分钟" },
];

export const credentialOperationLogs: CredentialOperationLog[] = [
  { id: "1", action: "更新财联社 Cookie", status: "success", time: "15:28:41" },
  { id: "2", action: "测试 Alpha Vantage Key", status: "success", time: "15:20:13" },
  { id: "3", action: "清除雪球过期凭据", status: "success", time: "14:55:02" },
];
