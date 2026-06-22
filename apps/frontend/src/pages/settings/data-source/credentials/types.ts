export type CredentialAuthType = "none" | "api_key" | "cookie" | "bearer_token" | "custom_header";

export type CredentialStatus = "normal" | "not_configured" | "expired" | "expiring" | "failed";

export type DataSourceProvider = {
  id: string;
  name: string;
  capability: string;
  status: CredentialStatus;
  authType: CredentialAuthType;
  iconType: string;
};

export type CredentialConfig = {
  providerId: string;
  providerName: string;
  capability: string;
  authType: CredentialAuthType;
  baseUrl: string;
  credentialStatus: CredentialStatus;
  expiresAt?: string;
  timeoutSeconds: number;
  rateLimitPerMinute: number;
  maskedCredential: string;
  note?: string;
};

export type CredentialTestTarget = {
  label: string;
  value: string;
};

export type CredentialTestResult = {
  status: "success" | "failed" | "untested";
  responseTimeMs?: number;
  testedAt?: string;
  messages: string[];
};

export type CredentialOverview = {
  configuredCount: number;
  expiringSoonCount: number;
  expiredCount: number;
};

export type CredentialHealthItem = {
  name: string;
  status: "normal" | "limited" | "failed";
  rateLimitText: string;
};

export type CredentialOperationLog = {
  id: string;
  action: string;
  status: "success" | "failed";
  time: string;
};
