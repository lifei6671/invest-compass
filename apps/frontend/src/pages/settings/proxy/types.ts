export type ProxyMode = "system" | "http" | "socks5";

export type ProxyModeOption = {
  mode: ProxyMode;
  title: string;
  description: string;
  recommended?: boolean;
  current?: boolean;
};

export type SystemProxyStatus = {
  source: string;
  enabled: boolean;
  pacMode: string;
  proxyAddress: string;
  bypassAddress: string;
  lastCheckedAt: string;
};

export type ProxyTestTarget =
  | "baidu"
  | "google"
  | "openai"
  | "deepseek";

export type ProxyTestResult = {
  status: "success" | "failed" | "untested";
  responseTimeMs?: number;
  checkedAt?: string;
};

export type HttpProxyProtocol = "http-https" | "http-only" | "https-only";
export type SocksProxyVersion = "socks5" | "socks4";

export type HttpProxyConfig = {
  host: string;
  port: number;
  protocol: HttpProxyProtocol;
  authenticationEnabled: boolean;
  username: string;
  password: string;
  timeoutSeconds: number;
};

export type Socks5ProxyConfig = {
  host: string;
  port: number;
  version: SocksProxyVersion;
  authenticationEnabled: boolean;
  username: string;
  password: string;
  timeoutSeconds: number;
};

export type ProxySettingsState = {
  proxyMode: ProxyMode;
  testTarget: ProxyTestTarget;
  bypassRules: string;
  testing: boolean;
  systemProxyStatus: SystemProxyStatus;
  httpProxyConfig: HttpProxyConfig;
  socks5ProxyConfig: Socks5ProxyConfig;
  testResult: ProxyTestResult;
};

export const proxyModeOptions: ProxyModeOption[] = [
  {
    mode: "system",
    title: "系统代理（推荐）",
    description: "使用系统网络设置（如系统代理或 PAC）",
    recommended: true,
    current: true,
  },
  {
    mode: "http",
    title: "HTTP 代理",
    description: "通过 HTTP 代理服务器访问外部网络",
  },
  {
    mode: "socks5",
    title: "SOCKS5 代理",
    description: "通过 SOCKS5 代理服务器访问外部网络",
  },
];

export const testTargetOptions: Array<{ label: ProxyTestTarget; value: ProxyTestTarget }> = [
  { label: "baidu", value: "baidu" },
  { label: "google", value: "google" },
  { label: "openai", value: "openai" },
  { label: "deepseek", value: "deepseek" },
];

export const httpProxyProtocolOptions: Array<{ label: string; value: HttpProxyProtocol }> = [
  { label: "HTTP / HTTPS", value: "http-https" },
  { label: "仅 HTTP", value: "http-only" },
  { label: "仅 HTTPS", value: "https-only" },
];

export const socksProxyVersionOptions: Array<{ label: string; value: SocksProxyVersion }> = [
  { label: "SOCKS5", value: "socks5" },
  { label: "SOCKS4", value: "socks4" },
];

export const initialHttpProxyConfig: HttpProxyConfig = {
  host: "127.0.0.1",
  port: 7890,
  protocol: "http-https",
  authenticationEnabled: false,
  username: "",
  password: "",
  timeoutSeconds: 10,
};

export const initialSocks5ProxyConfig: Socks5ProxyConfig = {
  host: "127.0.0.1",
  port: 1080,
  version: "socks5",
  authenticationEnabled: false,
  username: "",
  password: "",
  timeoutSeconds: 10,
};

export const initialProxyState: ProxySettingsState = {
  proxyMode: "system",
  testTarget: "baidu",
  bypassRules: "",
  testing: false,
  systemProxyStatus: {
    source: "操作系统",
    enabled: true,
    pacMode: "自动检测",
    proxyAddress: "根据系统设置",
    bypassAddress: "根据系统设置",
    lastCheckedAt: "2025-05-20 15:30:00",
  },
  httpProxyConfig: initialHttpProxyConfig,
  socks5ProxyConfig: initialSocks5ProxyConfig,
  testResult: {
    status: "success",
    responseTimeMs: 128,
    checkedAt: "2025-05-20 15:30:00",
  },
};
