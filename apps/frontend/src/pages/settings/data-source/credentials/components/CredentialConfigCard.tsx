import { EyeInvisibleOutlined } from "@ant-design/icons";
import { Button, DatePicker, Input, InputNumber, Select } from "antd";
import type { ReactNode } from "react";
import { appDatePickerLocale } from "../../../../../lib/antdLocale";
import type { CredentialAuthType, CredentialConfig } from "../types";
import { authTypeText, CredentialStatusTag } from "./CredentialStatusTag";

type CredentialConfigCardProps = {
  value: CredentialConfig;
  onChange: (value: CredentialConfig) => void;
  onAuthTypeChange: (value: CredentialAuthType) => void;
  onSave: () => void;
  onTest: () => void;
  onClear: () => void;
  testing: boolean;
};

const authTypeOptions: Array<{ label: string; value: CredentialAuthType }> = [
  { label: "无需凭据", value: "none" },
  { label: "API Key", value: "api_key" },
  { label: "Cookie", value: "cookie" },
  { label: "Bearer Token", value: "bearer_token" },
  { label: "Custom Header", value: "custom_header" },
];

export function CredentialConfigCard(props: CredentialConfigCardProps) {
  const update = <Key extends keyof CredentialConfig>(key: Key, value: CredentialConfig[Key]) => {
    props.onChange({ ...props.value, [key]: value });
  };
  const noteLength = props.value.note?.length ?? 0;

  return (
    <section className="credential-card credential-config-card">
      <h2>凭据配置</h2>
      <div className="credential-form">
        <Field label="Provider 名称" htmlFor="credential-provider-name">
          <Input id="credential-provider-name" className="credential-input" value={props.value.providerName} onChange={(event) => update("providerName", event.target.value)} />
        </Field>
        <Field label="数据能力">
          <span className="credential-static-value">{props.value.capability}</span>
        </Field>
        <Field label="认证方式">
          <Select<CredentialAuthType> className="credential-select" value={props.value.authType} options={authTypeOptions} onChange={props.onAuthTypeChange} />
        </Field>
        <Field label="Base URL" htmlFor="credential-base-url">
          <Input id="credential-base-url" className="credential-input" value={props.value.baseUrl} onChange={(event) => update("baseUrl", event.target.value)} />
        </Field>
        <Field label="凭据状态">
          <CredentialStatusTag status={props.value.credentialStatus} text={props.value.credentialStatus === "normal" ? "已配置" : undefined} />
        </Field>
        <Field label="过期时间" htmlFor="credential-expires-at">
          <DatePicker
            id="credential-expires-at"
            key={`${props.value.providerId}-${props.value.expiresAt ?? ""}`}
            className="credential-date-picker"
            format="YYYY-MM-DD HH:mm"
            locale={appDatePickerLocale}
            showTime={{ format: "HH:mm" }}
            inputReadOnly={false}
            placeholder={props.value.expiresAt || "请选择过期时间"}
            onChange={(_, dateString) => update("expiresAt", Array.isArray(dateString) ? dateString[0] : dateString)}
          />
        </Field>
        <Field label="请求超时">
          <div className="credential-number-addon">
            <InputNumber className="credential-number" min={1} max={120} value={props.value.timeoutSeconds} onChange={(value) => update("timeoutSeconds", Number(value ?? 1))} />
            <span>秒</span>
          </div>
        </Field>
        <Field label="频率限制">
          <div className="credential-number-addon">
            <InputNumber className="credential-number" min={1} max={300} value={props.value.rateLimitPerMinute} onChange={(value) => update("rateLimitPerMinute", Number(value ?? 1))} />
            <span>次/分钟</span>
          </div>
        </Field>
        <Field label={`${authTypeText(props.value.authType)} / 凭据内容`} htmlFor="credential-masked-content" alignStart>
          <div className="credential-textarea-wrap">
            <Input.TextArea id="credential-masked-content" className="credential-textarea credential-secret-textarea" value={props.value.maskedCredential} autoSize={false} onChange={(event) => update("maskedCredential", event.target.value)} />
            <EyeInvisibleOutlined className="credential-eye-icon" />
          </div>
          <p className="credential-helper">保存后仅展示脱敏状态，明文不在前端回显。</p>
        </Field>
        <Field label="备注（可选）" htmlFor="credential-note" alignStart>
          <Input.TextArea id="credential-note" className="credential-textarea credential-note-textarea" placeholder="请输入备注信息（可选）" maxLength={200} value={props.value.note ?? ""} onChange={(event) => update("note", event.target.value)} />
          <div className="credential-counter">{noteLength} / 200</div>
        </Field>
      </div>
      <div className="credential-action-row">
        <Button type="primary" className="credential-primary-button" onClick={props.onSave}>
          保存凭据
        </Button>
        <Button className="credential-outline-button" loading={props.testing} onClick={props.onTest}>
          测试连接
        </Button>
        <Button className="credential-muted-button" onClick={props.onClear}>
          清除凭据
        </Button>
      </div>
    </section>
  );
}

function Field(props: { label: string; htmlFor?: string; alignStart?: boolean; children: ReactNode }) {
  return (
    <div className={["credential-field", props.alignStart ? "credential-field-start" : ""].join(" ")}>
      <label htmlFor={props.htmlFor}>{props.label}</label>
      <div>{props.children}</div>
    </div>
  );
}
