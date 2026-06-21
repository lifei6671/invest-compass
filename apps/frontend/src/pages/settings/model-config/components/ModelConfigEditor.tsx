import { Input, InputNumber, Select, Switch } from "antd";
import type { ReactNode } from "react";
import type { ModelConfig, ModelConfigDraft } from "../types";
import { providers } from "../mock";

type ModelConfigEditorProps = {
  draft: ModelConfigDraft;
  selectedConfig: ModelConfig | null;
  onDraftChange: (draft: ModelConfigDraft) => void;
};

export function ModelConfigEditor(props: ModelConfigEditorProps) {
  const maskedKey = props.draft.apiKeyInput ?? (props.selectedConfig?.keyStatus.hasKey ? "••••••••••••••••••••••••••••••" : "");
  return (
    <section className="max-h-[60vh] overflow-y-auto px-6 py-4">
      <Field label="配置名称">
        <Input className="h-8 rounded-md border-[#d9e2f1]" value={props.draft.name} onChange={(event) => props.onDraftChange({ ...props.draft, name: event.target.value })} />
      </Field>
      <Field label="Provider">
        <Select
          className="h-8 w-full"
          value={props.draft.provider}
          options={providers.map((item) => ({ value: item.id, label: item.name }))}
          onChange={(provider) => {
            const meta = providers.find((item) => item.id === provider);
            props.onDraftChange({ ...props.draft, provider, baseUrl: meta?.defaultBaseUrl ?? props.draft.baseUrl });
          }}
        />
      </Field>
      <Field label="Base URL">
        <Input className="h-8 rounded-md border-[#d9e2f1]" value={props.draft.baseUrl} onChange={(event) => props.onDraftChange({ ...props.draft, baseUrl: event.target.value })} />
      </Field>
      <Field label="API Key">
        <Input.Password
          className="h-8 rounded-md border-[#d9e2f1]"
          value={maskedKey}
          onFocus={() => {
            if (props.draft.apiKeyInput === undefined) {
              props.onDraftChange({ ...props.draft, apiKeyInput: "" });
            }
          }}
          onChange={(event) => props.onDraftChange({ ...props.draft, apiKeyInput: event.target.value })}
        />
      </Field>
      <Field label="模型名">
        <Input className="h-8 rounded-md border-[#d9e2f1]" value={props.draft.modelName} onChange={(event) => props.onDraftChange({ ...props.draft, modelName: event.target.value })} />
      </Field>
      <NumberField label="温度 (Temperature)" min={0} max={2} step={0.1} precision={2} value={props.draft.temperature} onChange={(value) => props.onDraftChange({ ...props.draft, temperature: value })} />
      <NumberField label="最大输出 Token" min={256} max={32768} step={256} value={props.draft.maxTokens} onChange={(value) => props.onDraftChange({ ...props.draft, maxTokens: value })} />
      <NumberField label="超时时间（秒）" min={10} max={600} step={10} value={props.draft.timeoutSeconds} onChange={(value) => props.onDraftChange({ ...props.draft, timeoutSeconds: value })} />
      <SwitchRow label="启用流式输出" checked={props.draft.streamEnabled} onChange={(checked) => props.onDraftChange({ ...props.draft, streamEnabled: checked })} />
      <SwitchRow label="设为默认模型" checked={props.draft.isDefault} onChange={(checked) => props.onDraftChange({ ...props.draft, isDefault: checked })} />
    </section>
  );
}

function Field(props: { label: string; children: ReactNode }) {
  return (
    <label className="mb-3 block">
      <span className="mb-1.5 block text-[12px] font-semibold text-[#374151]">{props.label}</span>
      {props.children}
    </label>
  );
}

function NumberField(props: { label: string; value: number; min: number; max: number; step: number; precision?: number; onChange: (value: number) => void }) {
  const clamp = (value: number) => Math.min(props.max, Math.max(props.min, value));
  return (
    <Field label={props.label}>
      <InputNumber
        className="model-config-number-input h-8 w-full rounded-md border-[#d9e2f1]"
        min={props.min}
        max={props.max}
        step={props.step}
        precision={props.precision}
        value={props.value}
        onChange={(value) => props.onChange(clamp(Number(value ?? props.min)))}
      />
    </Field>
  );
}

function SwitchRow(props: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <div className="mb-3 flex h-8 items-center justify-between border-t border-[#edf1f7] pt-3">
      <span className="text-[12px] font-semibold text-[#374151]">{props.label}</span>
      <Switch size="small" checked={props.checked} onChange={props.onChange} />
    </div>
  );
}
