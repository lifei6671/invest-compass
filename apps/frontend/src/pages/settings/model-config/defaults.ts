import type { AIProviderType, ModelConfig, ModelConfigDraft, ProviderItem, SettingsTabKey } from "./types";

export const settingsTabs: Array<{ key: SettingsTabKey; label: string }> = [
  { key: "basic", label: "基础设置" },
  { key: "model-config", label: "模型配置" },
  { key: "data-source", label: "数据源设置" },
  { key: "proxy", label: "代理设置" },
  { key: "notification", label: "通知设置" },
  { key: "workspace", label: "工作区设置" },
  { key: "cache", label: "缓存管理" },
  { key: "about", label: "关于" },
];

export const providers: ProviderItem[] = [
  { id: "openai-compatible", name: "OpenAI Compatible", icon: "openai", defaultBaseUrl: "" },
  { id: "deepseek", name: "DeepSeek", icon: "deepseek", defaultBaseUrl: "" },
  { id: "qwen", name: "Qwen", icon: "qwen", defaultBaseUrl: "" },
  { id: "doubao-ark", name: "Doubao / Ark", icon: "doubao", defaultBaseUrl: "" },
  { id: "gemini", name: "Gemini", icon: "gemini", defaultBaseUrl: "" },
  { id: "claude", name: "Claude", icon: "claude", defaultBaseUrl: "" },
  { id: "ollama", name: "Ollama", icon: "ollama", defaultBaseUrl: "" },
  { id: "lm-studio", name: "LM Studio", icon: "lm-studio", defaultBaseUrl: "" },
  { id: "custom-http-provider", name: "Custom HTTP Provider", icon: "llm-api", defaultBaseUrl: "" },
];

export const initialModelConfigs: ModelConfig[] = [];

export function providerName(provider: AIProviderType) {
  return providers.find((item) => item.id === provider)?.name ?? "OpenAI Compatible";
}

export function defaultDraft(provider: AIProviderType): ModelConfigDraft {
  const item = providers.find((value) => value.id === provider) ?? providers[0];
  return {
    name: "",
    provider: item.id,
    baseUrl: item.defaultBaseUrl,
    modelName: "",
    temperature: 0.7,
    maxTokens: 4096,
    timeoutSeconds: 60,
    streamEnabled: true,
    isDefault: false,
  };
}

export function draftFromConfig(config: ModelConfig): ModelConfigDraft {
  return {
    id: config.id,
    name: config.name,
    provider: config.provider,
    baseUrl: config.baseUrl,
    modelName: config.modelName,
    temperature: config.temperature,
    maxTokens: config.maxTokens,
    timeoutSeconds: config.timeoutSeconds,
    streamEnabled: config.streamEnabled,
    isDefault: config.isDefault,
  };
}
