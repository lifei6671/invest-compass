export type AIProviderType =
  | "openai-compatible"
  | "deepseek"
  | "qwen"
  | "doubao-ark"
  | "gemini"
  | "claude"
  | "ollama"
  | "lm-studio"
  | "custom-http-provider";

export type ProviderIconKey =
  | "openai"
  | "deepseek"
  | "qwen"
  | "doubao"
  | "gemini"
  | "claude"
  | "ollama"
  | "lm-studio"
  | "llm-api";

export type ConnectionStatus = "normal" | "untested" | "failed" | "testing";

export type KeyStatus = {
  hasKey: boolean;
  maskedKey?: string;
};

export type ProviderItem = {
  id: AIProviderType;
  name: string;
  icon: ProviderIconKey;
  defaultBaseUrl: string;
};

export type ModelConfig = {
  id: string;
  apiKeyRef: string;
  name: string;
  provider: AIProviderType;
  providerName: string;
  baseUrl: string;
  modelName: string;
  streamEnabled: boolean;
  isDefault: boolean;
  keyStatus: KeyStatus;
  connectionStatus: ConnectionStatus;
  testDurationMs?: number;
  temperature: number;
  maxTokens: number;
  timeoutSeconds: number;
};

export type ModelConfigDraft = {
  id?: string;
  name: string;
  provider: AIProviderType;
  baseUrl: string;
  apiKeyInput?: string;
  modelName: string;
  temperature: number;
  maxTokens: number;
  timeoutSeconds: number;
  streamEnabled: boolean;
  isDefault: boolean;
};

export type SettingsTabKey =
  | "basic"
  | "model-config"
  | "data-source"
  | "proxy"
  | "notification"
  | "workspace"
  | "cache"
  | "about";
