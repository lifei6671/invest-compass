import { App as AntApp, Button, Modal } from "antd";
import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ModelConfigEditor } from "./components/ModelConfigEditor";
import { ModelConfigTable } from "./components/ModelConfigTable";
import { ProviderListPanel } from "./components/ProviderListPanel";
import { SettingsTabs } from "./components/SettingsTabs";
import { defaultDraft, draftFromConfig, providerName } from "./defaults";
import { aiConfigDelete, aiConfigList, aiConfigSave, aiConfigTest, type AIConfig, type AIConfigSavePayload } from "../../../services/coreClient";
import { useDashboardStore } from "../../../stores/dashboardStore";
import type { AIProviderType, ConnectionStatus, ModelConfig, ModelConfigDraft, SettingsTabKey } from "./types";

type ModelConfigPageProps = {
  showTabs?: boolean;
};

const connectionStatusCache = new Map<string, ConnectionStatus>();

export function ModelConfigPage(props: ModelConfigPageProps = {}) {
  const { message, modal } = AntApp.useApp();
  const showTabs = props.showTabs !== false;
  const [activeSettingsTab, setActiveSettingsTab] = useState<SettingsTabKey>("model-config");
  const [selectedProvider, setSelectedProvider] = useState<AIProviderType>("openai-compatible");
  const [configs, setConfigs] = useState<ModelConfig[]>([]);
  const [selectedConfigId, setSelectedConfigId] = useState<string | null>(null);
  const [draft, setDraft] = useState<ModelConfigDraft>(() => defaultDraft("openai-compatible"));
  const [editorMode, setEditorMode] = useState<"create" | "edit">("create");
  const [editorOpen, setEditorOpen] = useState(false);
  const [savingConfig, setSavingConfig] = useState(false);
  const [testingEditor, setTestingEditor] = useState(false);
  const [testingConfigId, setTestingConfigId] = useState<string | null>(null);
  const testingConfigIdRef = useRef<string | null>(null);
  const reportCoreError = useCallback((error: unknown) => {
    const messageText = safeErrorMessage(error, "");
    if (!isCoreUnavailableMessage(messageText)) {
      return;
    }
    useDashboardStore.setState({
      state: null,
      error: messageText || "本地核心服务连接失败",
    });
  }, []);

  const filteredConfigs = useMemo(() => configs.filter((item) => item.provider === selectedProvider), [configs, selectedProvider]);
  const selectedConfig = useMemo(() => configs.find((item) => item.id === selectedConfigId) ?? null, [configs, selectedConfigId]);

  const loadConfigs = useCallback(async () => {
    try {
      const result = await aiConfigList();
      const nextConfigs = result.items.map(modelConfigFromAIConfig);
      const firstConfig = nextConfigs[0] ?? null;
      setConfigs(nextConfigs);
      setSelectedConfigId((current) => current ?? firstConfig?.id ?? null);
      setDraft((current) => {
        if (current.id) {
          return current;
        }
        return firstConfig ? draftFromConfig(firstConfig) : current;
      });
      setSelectedProvider((current) => {
        if (nextConfigs.some((item) => item.provider === current)) {
          return current;
        }
        return nextConfigs[0]?.provider ?? current;
      });
    } catch (error) {
      setConfigs([]);
      setSelectedConfigId(null);
      reportCoreError(error);
      message.error(safeErrorMessage(error, "模型配置读取失败"));
    }
  }, [message, reportCoreError]);

  useEffect(() => {
    void loadConfigs();
  }, [loadConfigs]);

  const selectProvider = (provider: AIProviderType) => {
    const first = configs.find((item) => item.provider === provider);
    setSelectedProvider(provider);
    setSelectedConfigId(first?.id ?? null);
    setDraft(first ? draftFromConfig(first) : defaultDraft(provider));
    setEditorMode(first ? "edit" : "create");
  };

  const selectConfig = (config: ModelConfig) => {
    setSelectedProvider(config.provider);
    setSelectedConfigId(config.id);
    setDraft(draftFromConfig(config));
    setEditorMode("edit");
  };

  const createConfig = () => {
    setSelectedConfigId(null);
    setDraft(defaultDraft(selectedProvider));
    setEditorMode("create");
    setEditorOpen(true);
  };

  const editConfig = (config: ModelConfig) => {
    selectConfig(config);
    setEditorOpen(true);
  };

  const saveConfig = async () => {
    const validationError = validateDraft(draft);
    if (validationError) {
      message.warning(validationError);
      return;
    }
    const payload = aiConfigPayloadFromDraft(draft, selectedConfig);
    setSavingConfig(true);
    try {
      const saved = await aiConfigSave(payload).then((result) => result.config);
      const nextConfig = modelConfigFromAIConfig(saved);
      setConfigs((current) => upsertModelConfig(current, nextConfig));
      setSelectedProvider(nextConfig.provider);
      setSelectedConfigId(nextConfig.id);
      setDraft(draftFromConfig(nextConfig));
      setEditorMode("edit");
      setEditorOpen(false);
      message.success("配置已保存");
    } catch (error) {
      reportCoreError(error);
      message.error(safeErrorMessage(error, "配置保存失败"));
    } finally {
      setSavingConfig(false);
    }
  };

  const deleteConfig = async (id: string) => {
    const numericId = Number(id);
    if (!Number.isInteger(numericId) || numericId <= 0) {
      message.error("配置 ID 无效");
      return;
    }
    try {
      await aiConfigDelete({ id: numericId });
      connectionStatusCache.delete(id);
      setConfigs((current) => current.filter((item) => item.id !== id));
      if (selectedConfigId === id) {
        setSelectedConfigId(null);
        setDraft(defaultDraft(selectedProvider));
        setEditorMode("create");
      }
      setEditorOpen(false);
      message.success("配置已删除");
    } catch (error) {
      reportCoreError(error);
      message.error(safeErrorMessage(error, "配置删除失败"));
    }
  };

  const setDefault = async (id: string) => {
    const config = configs.find((item) => item.id === id);
    if (!config) {
      return;
    }
    const nextDefault = !config.isDefault;
    try {
      const saved = await aiConfigSave(aiConfigPayloadFromConfig({ ...config, isDefault: nextDefault })).then((result) => result.config);
      const nextConfig = modelConfigFromAIConfig(saved);
      setConfigs((current) => upsertModelConfig(current, nextConfig));
      setSelectedConfigId(nextConfig.id);
      setDraft(draftFromConfig(nextConfig));
      message.success(nextDefault ? "已设为默认模型" : "已取消默认模型");
    } catch (error) {
      reportCoreError(error);
      message.error(safeErrorMessage(error, "默认模型保存失败"));
    }
  };

  const toggleStream = async (id: string, enabled: boolean) => {
    const config = configs.find((item) => item.id === id);
    if (!config) {
      return;
    }
    try {
      const saved = await aiConfigSave(aiConfigPayloadFromConfig({ ...config, streamEnabled: enabled })).then((result) => result.config);
      const nextConfig = modelConfigFromAIConfig(saved);
      setConfigs((current) => upsertModelConfig(current, nextConfig));
      if (draft.id === id) {
        setDraft(draftFromConfig(nextConfig));
      }
      message.success("配置已保存");
    } catch (error) {
      reportCoreError(error);
      message.error(safeErrorMessage(error, "配置保存失败"));
    }
  };

  const testConfig = async (id: string) => {
    if (testingConfigIdRef.current) {
      message.warning("已有连接测试进行中，请稍后再试");
      return;
    }
    const config = configs.find((item) => item.id === id);
    setSelectedConfigId(id);
    if (!config) {
      message.error("配置不存在");
      return;
    }
    const numericId = Number(id);
    if (!Number.isInteger(numericId) || numericId <= 0) {
      message.error("配置 ID 无效");
      return;
    }
    if (!config.apiKeyRef) {
      setConfigs((current) => current.map((item) => (item.id === id ? { ...item, connectionStatus: "failed" } : item)));
      message.warning("请先保存 API Key 后再测试连接");
      return;
    }
    testingConfigIdRef.current = id;
    rememberConnectionStatus(id, "testing");
    setTestingConfigId(id);
    setConfigs((current) => current.map((item) => (item.id === id ? { ...item, connectionStatus: "testing" } : item)));
    try {
      const result = await aiConfigTest({ id: numericId, api_key_ref: config.apiKeyRef });
      const nextStatus = result.ok ? "normal" : "failed";
      rememberConnectionStatus(id, nextStatus);
      setConfigs((current) => current.map((item) => (item.id === id ? { ...item, connectionStatus: nextStatus } : item)));
      if (result.ok) {
        message.success("连接测试完成");
        return;
      }
      message.error(redactSensitiveText(result.message || "连接测试失败"));
    } catch (error) {
      rememberConnectionStatus(id, "failed");
      setConfigs((current) => current.map((item) => (item.id === id ? { ...item, connectionStatus: "failed" } : item)));
      reportCoreError(error);
      message.error(safeErrorMessage(error, "连接测试失败"));
    } finally {
      if (testingConfigIdRef.current === id) {
        testingConfigIdRef.current = null;
        setTestingConfigId(null);
      }
    }
  };

  const testEditor = async () => {
    if (!draft.id) {
      message.warning("请先保存配置后再测试连接");
      return;
    }
    setTestingEditor(true);
    try {
      await testConfig(draft.id);
    } finally {
      setTestingEditor(false);
    }
  };

  return (
    <section className={[showTabs ? "-mt-3 min-h-[calc(100vh-108px)]" : "min-h-[calc(100vh-178px)]", "flex flex-col gap-3"].join(" ")}>
      {showTabs ? (
        <SettingsTabs
          activeKey={activeSettingsTab}
          onChange={(key) => {
            setActiveSettingsTab(key);
            if (key !== "model-config") {
              message.info("该设置页待接入");
            }
          }}
        />
      ) : null}
      <div className="grid min-h-[690px] flex-1 grid-cols-[260px_minmax(0,1fr)] gap-3">
        <ProviderListPanel selectedProvider={selectedProvider} onSelect={selectProvider} />
        <ModelConfigTable
          providerName={providerName(selectedProvider)}
          configs={filteredConfigs}
          selectedConfigId={selectedConfigId}
          testingConfigId={testingConfigId}
          onCreate={createConfig}
          onSelect={selectConfig}
          onEdit={editConfig}
          onToggleStream={toggleStream}
          onSetDefault={setDefault}
          onTest={testConfig}
          onDelete={deleteConfig}
        />
      </div>
      <Modal
        centered
        className="model-config-editor-modal"
        destroyOnHidden
        title={editorMode === "create" ? "新建配置" : "编辑配置"}
        footer={[
          <Button key="save" type="primary" className="h-8 min-w-[112px] rounded-md bg-[#1677ff]" loading={savingConfig} onClick={saveConfig}>
            保存配置
          </Button>,
          <Button key="test" className="h-8 min-w-[112px] rounded-md border-[#1677ff] text-[#1677ff]" loading={testingEditor} onClick={testEditor}>
            测试连接
          </Button>,
          <Button
            key="delete"
            className="h-8 min-w-[112px] rounded-md border-[#ffccc7] text-[#ff4d4f]"
            disabled={!draft.id}
            onClick={() => {
              modal.confirm({
                title: "确认删除配置？",
                content: `删除后将从本地配置列表移除「${draft.name || providerName(draft.provider)}」。`,
                okText: "删除",
                okButtonProps: { danger: true },
                cancelText: "取消",
                onOk: () => {
                  if (draft.id) {
                    void deleteConfig(draft.id);
                  }
                },
              });
            }}
          >
            删除配置
          </Button>,
        ]}
        mask={{ closable: true }}
        open={editorOpen}
        width={520}
        onCancel={() => setEditorOpen(false)}
      >
        <ModelConfigEditor draft={draft} selectedConfig={selectedConfig} onDraftChange={setDraft} />
      </Modal>
      <ModelConfigNotice />
    </section>
  );
}

function modelConfigFromAIConfig(config: AIConfig): ModelConfig {
  const provider = normalizeProvider(config.provider);
  const id = String(config.id);
  return {
    id,
    apiKeyRef: config.api_key_ref,
    name: config.name,
    provider,
    providerName: providerName(provider),
    baseUrl: config.base_url,
    modelName: config.model_name,
    streamEnabled: config.stream_enabled,
    isDefault: config.is_default,
    keyStatus: {
      hasKey: config.has_api_key,
      maskedKey: config.masked_api_key,
    },
    connectionStatus: connectionStatusCache.get(id) ?? "untested",
    temperature: config.temperature,
    maxTokens: config.max_tokens,
    timeoutSeconds: config.timeout_seconds,
  };
}

function rememberConnectionStatus(id: string, status: ConnectionStatus) {
  connectionStatusCache.set(id, status);
}

function aiConfigPayloadFromDraft(draft: ModelConfigDraft, selectedConfig: ModelConfig | null): AIConfigSavePayload {
  const apiKey = draft.apiKeyInput?.trim() ?? "";
  return {
    id: Number(draft.id ?? 0),
    name: draft.name.trim(),
    provider: draft.provider,
    base_url: draft.baseUrl.trim(),
    api_key_ref: selectedConfig?.apiKeyRef ?? "",
    masked_api_key: selectedConfig?.keyStatus.maskedKey ?? "",
    has_api_key: Boolean(selectedConfig?.keyStatus.hasKey || apiKey),
    model_name: draft.modelName.trim(),
    temperature: draft.temperature,
    max_tokens: draft.maxTokens,
    timeout_seconds: draft.timeoutSeconds,
    stream_enabled: draft.streamEnabled,
    is_default: draft.isDefault,
    api_key: apiKey || undefined,
  };
}

function aiConfigPayloadFromConfig(config: ModelConfig): AIConfigSavePayload {
  return {
    id: Number(config.id),
    name: config.name,
    provider: config.provider,
    base_url: config.baseUrl,
    api_key_ref: config.apiKeyRef,
    masked_api_key: config.keyStatus.maskedKey ?? "",
    has_api_key: config.keyStatus.hasKey,
    model_name: config.modelName,
    temperature: config.temperature,
    max_tokens: config.maxTokens,
    timeout_seconds: config.timeoutSeconds,
    stream_enabled: config.streamEnabled,
    is_default: config.isDefault,
  };
}

function upsertModelConfig(configs: ModelConfig[], config: ModelConfig) {
  const normalized = config.isDefault ? configs.map((item) => ({ ...item, isDefault: false })) : configs;
  if (normalized.some((item) => item.id === config.id)) {
    return normalized.map((item) => (item.id === config.id ? config : item));
  }
  return [config, ...normalized];
}

function validateDraft(draft: ModelConfigDraft) {
  if (!draft.name.trim()) {
    return "请填写配置名称";
  }
  if (!draft.baseUrl.trim()) {
    return "请填写 Base URL";
  }
  if (!draft.modelName.trim()) {
    return "请填写模型名";
  }
  return "";
}

function normalizeProvider(provider: string): AIProviderType {
  const value = provider.trim();
  if (
    value === "openai-compatible" ||
    value === "deepseek" ||
    value === "qwen" ||
    value === "doubao-ark" ||
    value === "gemini" ||
    value === "claude" ||
    value === "ollama" ||
    value === "lm-studio" ||
    value === "custom-http-provider"
  ) {
    return value;
  }
  return "custom-http-provider";
}

function safeErrorMessage(error: unknown, fallback: string) {
  if (typeof error === "string") {
    return redactSensitiveText(error) || fallback;
  }
  if (error && typeof error === "object" && "message" in error) {
    const message = (error as { message?: unknown }).message;
    if (typeof message === "string" && message.trim()) {
      return redactSensitiveText(message);
    }
  }
  if (!(error instanceof Error) || !error.message) {
    return fallback;
  }
  return redactSensitiveText(error.message);
}

function isCoreUnavailableMessage(value: string) {
  return value.includes("core sidecar is not running") || value.includes("sidecar http error");
}

function redactSensitiveText(value: string) {
  return value
    .replace(/\bsk-[A-Za-z0-9._-]+/g, "sk-[已脱敏]")
    .replace(/(Authorization\s*:\s*)[^\s,;]+/gi, "$1[已脱敏]")
    .replace(/(Proxy-Authorization\s*:\s*)[^\s,;]+/gi, "$1[已脱敏]")
    .replace(/\b(api[_-]?key|token|secret)=([^\s,;]+)/gi, "$1=[已脱敏]");
}

function ModelConfigNotice() {
  return (
    <div className="flex min-h-[42px] items-center justify-between rounded-lg border border-[#b9d6ff] bg-[#f2f7ff] px-5 text-[14px] text-[#155ec8]">
      <div className="flex items-center gap-3 font-medium">
        <InfoCircleOutlined className="text-[18px]" />
        API Key 以脱敏方式展示，敏感信息不在前端明文回显。
      </div>
      <div className="flex items-center gap-2 text-slate-500">
        <SafetyCertificateOutlined />
        仅供研究，不构成投资建议。
      </div>
    </div>
  );
}
