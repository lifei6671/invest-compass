import { Select, Switch } from "antd";
import { ClockCircleOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type {
  DataSourceBaseSettings,
  KlineRange,
  MarketScope,
  MarketSource,
  NewsSource,
  NewsSyncInterval,
  QuoteRefreshInterval,
} from "../types";

type DataSourceBaseSettingsCardProps = {
  value: DataSourceBaseSettings;
  onChange: (value: DataSourceBaseSettings) => void;
};

const marketSourceOptions: Array<{ label: string; value: MarketSource }> = [
  { label: "自动降级", value: "auto-fallback" },
  { label: "AkShare / EastMoney", value: "akshare-eastmoney" },
  { label: "EastMoney", value: "eastmoney" },
  { label: "新浪财经", value: "sina" },
  { label: "腾讯财经", value: "tencent" },
  { label: "Custom Provider", value: "custom" },
];

const newsSourceOptions: Array<{ label: string; value: NewsSource }> = [
  { label: "聚合新闻源", value: "aggregated" },
  { label: "财联社", value: "cls" },
  { label: "新浪财经", value: "sina" },
  { label: "自定义资讯源", value: "custom" },
];

const marketScopeOptions: Array<{ label: string; value: MarketScope }> = [
  { label: "A股", value: "CN" },
  { label: "港股", value: "HK" },
  { label: "美股", value: "US" },
  { label: "全部市场", value: "ALL" },
];

const klineRangeOptions: Array<{ label: string; value: KlineRange }> = [
  { label: "近 1 年", value: "1y" },
  { label: "近 3 年", value: "3y" },
  { label: "近 5 年", value: "5y" },
  { label: "全量可用数据", value: "all" },
];

const quoteRefreshOptions: Array<{ label: string; value: QuoteRefreshInterval }> = [
  { label: "15 秒", value: "15s" },
  { label: "30 秒", value: "30s" },
  { label: "60 秒", value: "60s" },
  { label: "120 秒", value: "120s" },
  { label: "手动刷新", value: "manual" },
];

const newsSyncOptions: Array<{ label: string; value: NewsSyncInterval }> = [
  { label: "5 分钟", value: "5m" },
  { label: "15 分钟", value: "15m" },
  { label: "30 分钟", value: "30m" },
  { label: "60 分钟", value: "60m" },
  { label: "手动同步", value: "manual" },
];

export function DataSourceBaseSettingsCard(props: DataSourceBaseSettingsCardProps) {
  const update = <Key extends keyof DataSourceBaseSettings>(key: Key, value: DataSourceBaseSettings[Key]) => {
    props.onChange({ ...props.value, [key]: value });
  };

  return (
    <section className="settings-basic-card settings-data-source-hero-card">
      <header className="settings-basic-card-header">
        <h2>数据源基础设置</h2>
        <p>配置行情、资讯与同步任务的数据来源及刷新策略</p>
      </header>
      <div className="settings-data-source-form-grid">
        <div className="settings-basic-form-column">
          <SelectField label="默认行情源" value={props.value.defaultMarketSource} options={marketSourceOptions} onChange={(value) => update("defaultMarketSource", value)} />
          <SelectField label="默认资讯源" value={props.value.defaultNewsSource} options={newsSourceOptions} onChange={(value) => update("defaultNewsSource", value)} />
          <SelectField label="默认市场范围" value={props.value.defaultMarketScope} options={marketScopeOptions} onChange={(value) => update("defaultMarketScope", value)} />
          <SelectField label="K线数据范围" value={props.value.klineRange} options={klineRangeOptions} onChange={(value) => update("klineRange", value)} />
        </div>
        <div className="settings-basic-form-column">
          <SelectField
            label="行情刷新频率"
            icon={<ClockCircleOutlined />}
            value={props.value.quoteRefreshInterval}
            options={quoteRefreshOptions}
            onChange={(value) => update("quoteRefreshInterval", value)}
          />
          <SelectField
            label="新闻同步频率"
            icon={<ClockCircleOutlined />}
            value={props.value.newsSyncInterval}
            options={newsSyncOptions}
            onChange={(value) => update("newsSyncInterval", value)}
          />
          <SwitchField label="启动时自动同步" checked={props.value.syncOnStartup} onChange={(checked) => update("syncOnStartup", checked)} />
          <SwitchField
            label="非交易时段降频"
            checked={props.value.reduceFrequencyOutsideTradingHours}
            onChange={(checked) => update("reduceFrequencyOutsideTradingHours", checked)}
          />
        </div>
      </div>
      <p className="settings-data-source-note">选择自动降级时，会按已启用数据源顺序尝试；前一数据源无数据或数据不完整时使用下一个数据源备份。</p>
    </section>
  );
}

function SelectField<Value extends string>(props: {
  label: string;
  icon?: ReactNode;
  value: Value;
  options: Array<{ label: string; value: Value }>;
  onChange: (value: Value) => void;
}) {
  return (
    <div className="settings-data-source-field">
      <label>{props.label}</label>
      <Select<Value>
        aria-label={props.label}
        className="settings-basic-select settings-data-source-select"
        prefix={props.icon}
        value={props.value}
        options={props.options}
        onChange={props.onChange}
      />
    </div>
  );
}

function SwitchField(props: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <div className="settings-data-source-field settings-data-source-switch-field">
      <label>{props.label}</label>
      <Switch checked={props.checked} onChange={props.onChange} />
    </div>
  );
}
