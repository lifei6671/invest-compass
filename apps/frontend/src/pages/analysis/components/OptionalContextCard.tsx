import { InfoCircleOutlined } from "@ant-design/icons";
import { Input, Select } from "antd";
import { riskPreferences } from "../defaults";
import type { OptionalHoldingContext, RiskPreference } from "../types";

type OptionalContextCardProps = {
  value: OptionalHoldingContext;
  onChange: (value: OptionalHoldingContext) => void;
};

export function OptionalContextCard(props: OptionalContextCardProps) {
  return (
    <section className="analysis-card analysis-optional-card">
      <header className="analysis-card-title-row">
        <h2 className="analysis-card-title">可选持仓上下文</h2>
        <InfoCircleOutlined />
      </header>
      <div className="analysis-form-stack analysis-context-form">
        <label className="analysis-inline-field">
          <span>成本价（元）</span>
          <Input
            className="analysis-input"
            placeholder="选填"
            value={props.value.costPrice}
            onChange={(event) => props.onChange({ ...props.value, costPrice: event.target.value })}
          />
        </label>
        <label className="analysis-inline-field">
          <span>股数（股）</span>
          <Input
            className="analysis-input"
            placeholder="选填"
            value={props.value.shares}
            onChange={(event) => props.onChange({ ...props.value, shares: event.target.value })}
          />
        </label>
        <label className="analysis-inline-field">
          <span>风险偏好</span>
          <Select
            className="analysis-control"
            value={props.value.riskPreference}
            options={riskPreferences.map((item) => ({ value: item, label: item }))}
            onChange={(riskPreference: RiskPreference) => props.onChange({ ...props.value, riskPreference })}
          />
        </label>
      </div>
      <p className="analysis-context-note">仅用于本次分析上下文，不落库</p>
    </section>
  );
}
