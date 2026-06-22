import { App as AntApp, Button, Modal } from "antd";
import { InfoCircleOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { useMemo, useState } from "react";
import { ModelConfigEditor } from "./components/ModelConfigEditor";
import { ModelConfigTable } from "./components/ModelConfigTable";
import { ProviderListPanel } from "./components/ProviderListPanel";
import { SettingsTabs } from "./components/SettingsTabs";
import { defaultDraft, draftFromConfig, initialModelConfigs, providerName } from "./defaults";
import type { AIProviderType, ModelConfig, ModelConfigDraft, SettingsTabKey } from "./types";

type ModelConfigPageProps = {
  showTabs?: boolean;
};

export function ModelConfigPage(props: ModelConfigPageProps = {}) {
  const { message, modal } = AntApp.useApp();
  const showTabs = props.showTabs !== false;
  const [activeSettingsTab, setActiveSettingsTab] = useState<SettingsTabKey>("model-config");
  const [selectedProvider, setSelectedProvider] = useState<AIProviderType>("openai-compatible");
  const [configs, setConfigs] = useState<ModelConfig[]>(initialModelConfigs);
  const [selectedConfigId, setSelectedConfigId] = useState<string | null>(null);
  const [draft, setDraft] = useState<ModelConfigDraft>(() => defaultDraft("openai-compatible"));
  const [editorMode, setEditorMode] = useState<"create" | "edit">("create");
  const [editorOpen, setEditorOpen] = useState(false);
  const [testingEditor, setTestingEditor] = useState(false);

  const filteredConfigs = useMemo(() => configs.filter((item) => item.provider === selectedProvider), [configs, selectedProvider]);
  const selectedConfig = useMemo(() => configs.find((item) => item.id === selectedConfigId) ?? null, [configs, selectedConfigId]);

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

  const editConfig = (config: ModelConfig) => {
    selectConfig(config);
    setEditorOpen(true);
  };

  const createConfig = () => {
    setSelectedConfigId(null);
    setDraft(defaultDraft(selectedProvider));
    setEditorMode("create");
    setEditorOpen(true);
  };

  const saveConfig = () => {
    const name = draft.name.trim();
    const baseUrl = draft.baseUrl.trim();
    const modelName = draft.modelName.trim();
    if (!name || !baseUrl || !modelName) {
      message.warning("请填写配置名称、Base URL 和模型名");
      return;
    }

    const id = draft.id ?? `model-${Date.now()}`;
    const existing = configs.find((item) => item.id === id) ?? null;
    const keyStatus = draft.apiKeyInput
      ? { hasKey: true, maskedKey: "sk-****local" }
      : existing?.keyStatus ?? { hasKey: false };
    const nextConfig: ModelConfig = {
      id,
      name,
      provider: draft.provider,
      providerName: providerName(draft.provider),
      baseUrl,
      modelName,
      streamEnabled: draft.streamEnabled,
      isDefault: draft.isDefault,
      keyStatus,
      connectionStatus: existing?.connectionStatus ?? "untested",
      temperature: draft.temperature,
      maxTokens: draft.maxTokens,
      timeoutSeconds: draft.timeoutSeconds,
    };

    setConfigs((current) => {
      const normalized = draft.isDefault ? current.map((item) => ({ ...item, isDefault: false })) : current;
      if (existing) {
        return normalized.map((item) => (item.id === id ? nextConfig : item));
      }
      return [nextConfig, ...normalized];
    });
    setSelectedProvider(draft.provider);
    setSelectedConfigId(id);
    setDraft(draftFromConfig(nextConfig));
    setEditorMode("edit");
    setEditorOpen(false);
    message.success("配置已保存");
  };

  const deleteConfig = (id: string) => {
    const remaining = configs.filter((item) => item.id !== id);
    const first = remaining.find((item) => item.provider === selectedProvider) ?? remaining[0] ?? null;
    setConfigs(remaining);
    setSelectedConfigId(first?.id ?? null);
    setDraft(first ? draftFromConfig(first) : defaultDraft(selectedProvider));
    setEditorMode(first ? "edit" : "create");
    setEditorOpen(false);
    message.success("配置已删除");
  };

  const setDefault = (id: string) => {
    setConfigs((current) => current.map((item) => ({ ...item, isDefault: item.id === id })));
    if (draft.id === id) {
      setDraft({ ...draft, isDefault: true });
    }
    message.success("已设为默认模型");
  };

  const toggleStream = (id: string, enabled: boolean) => {
    setConfigs((current) => current.map((item) => (item.id === id ? { ...item, streamEnabled: enabled } : item)));
    if (draft.id === id) {
      setDraft({ ...draft, streamEnabled: enabled });
    }
  };

  const testConfig = (id: string) => {
    setSelectedConfigId(id);
    message.info("连接测试接口待接入");
  };

  const testEditor = () => {
    setTestingEditor(false);
    message.info("连接测试接口待接入");
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
        title={editorMode === "create" ? "新建配置" : "编辑配置"}
        footer={[
          <Button key="save" type="primary" className="h-8 min-w-[112px] rounded-md bg-[#1677ff]" onClick={saveConfig}>
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
                    deleteConfig(draft.id);
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
        <ModelConfigEditor
          draft={draft}
          selectedConfig={selectedConfig}
          onDraftChange={setDraft}
        />
      </Modal>
      <ModelConfigNotice />
    </section>
  );
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
