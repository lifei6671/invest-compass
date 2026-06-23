import { Select } from "antd";
import {
  BarChartOutlined,
  ClockCircleOutlined,
  GlobalOutlined,
  LineChartOutlined,
  RobotOutlined,
  SettingOutlined,
  SunOutlined,
} from "@ant-design/icons";
import type { ReactNode } from "react";
import type { BasicSettingsState, AdjustType, KlinePeriod, Language, Market, RefreshInterval, ThemeMode } from "../types";

type AppBasicSettingsCardProps = {
  value: BasicSettingsState;
  aiModelOptions: Array<{ label: string; value: string }>;
  onChange: (value: BasicSettingsState) => void;
};

type SettingFieldProps<Value extends string> = {
  icon: ReactNode;
  label: string;
  value: Value;
  options: Array<{ label: string; value: Value }>;
  disabled?: boolean;
  note?: string;
  onChange: (value: Value) => void;
};

const themeOptions: Array<{ label: string; value: ThemeMode }> = [
  { label: "浅色模式", value: "light" },
  { label: "深色模式", value: "dark" },
  { label: "跟随系统", value: "system" },
];

const languageOptions: Array<{ label: string; value: Language }> = [
  { label: "简体中文", value: "zh-CN" },
  { label: "English", value: "en-US" },
];

const marketOptions: Array<{ label: string; value: Market }> = [
  { label: "A股", value: "CN" },
  { label: "港股", value: "HK" },
  { label: "美股", value: "US" },
];

const refreshOptions: Array<{ label: string; value: RefreshInterval }> = [
  { label: "15 秒", value: "15s" },
  { label: "30 秒", value: "30s" },
  { label: "60 秒", value: "60s" },
  { label: "120 秒", value: "120s" },
  { label: "手动刷新", value: "manual" },
];

const klineOptions: Array<{ label: string; value: KlinePeriod }> = [
  { label: "分时", value: "minute" },
  { label: "日线", value: "day" },
  { label: "周线", value: "week" },
  { label: "月线", value: "month" },
];

const adjustOptions: Array<{ label: string; value: AdjustType }> = [
  { label: "不复权", value: "none" },
  { label: "前复权", value: "qfq" },
  { label: "后复权", value: "hfq" },
];

export function AppBasicSettingsCard(props: AppBasicSettingsCardProps) {
  const update = <Key extends keyof BasicSettingsState>(key: Key, value: BasicSettingsState[Key]) => {
    props.onChange({ ...props.value, [key]: value });
  };

  return (
    <section className="settings-basic-card settings-basic-hero-card">
      <header className="settings-basic-card-header">
        <h2>应用基础设置</h2>
        <p>配置应用的基本行为与偏好设置</p>
      </header>
      <div className="settings-basic-form-grid">
        <div className="settings-basic-form-column">
          <SettingField icon={<SunOutlined />} label="主题" value={props.value.theme} options={themeOptions} onChange={(value) => update("theme", value)} />
          <SettingField icon={<GlobalOutlined />} label="语言" value={props.value.language} options={languageOptions} onChange={(value) => update("language", value)} />
          <SettingField icon={<BarChartOutlined />} label="默认市场" value={props.value.defaultMarket} options={marketOptions} onChange={(value) => update("defaultMarket", value)} />
          <SettingField
            icon={<RobotOutlined />}
            label="默认 AI 模型"
            value={props.value.defaultAIModel}
            options={props.aiModelOptions}
            disabled={props.aiModelOptions.length === 0}
            note="用于新建分析任务时的默认模型，可在模型配置中管理更多模型。"
            onChange={(value) => update("defaultAIModel", value)}
          />
        </div>
        <div className="settings-basic-form-column">
          <SettingField icon={<ClockCircleOutlined />} label="行情刷新频率" value={props.value.quoteRefreshInterval} options={refreshOptions} onChange={(value) => update("quoteRefreshInterval", value)} />
          <SettingField icon={<LineChartOutlined />} label="默认 K 线周期" value={props.value.defaultKlinePeriod} options={klineOptions} onChange={(value) => update("defaultKlinePeriod", value)} />
          <SettingField icon={<SettingOutlined />} label="默认复权类型" value={props.value.defaultAdjustType} options={adjustOptions} onChange={(value) => update("defaultAdjustType", value)} />
        </div>
      </div>
    </section>
  );
}

function SettingField<Value extends string>(props: SettingFieldProps<Value>) {
  return (
    <div className="settings-basic-field">
      <label>
        <span className="settings-basic-field-icon">{props.icon}</span>
        <span>{props.label}</span>
      </label>
      <div className="settings-basic-field-control">
        <Select<Value>
          className="settings-basic-select"
          value={props.value}
          options={props.options}
          disabled={props.disabled}
          onChange={props.onChange}
        />
        {props.note ? <p>{props.note}</p> : null}
      </div>
    </div>
  );
}
