import type { CredentialOverview } from "../types";

export function CredentialOverviewCard({ value }: { value: CredentialOverview }) {
  return (
    <section className="credential-card credential-bottom-card">
      <h2>已配置凭据概览</h2>
      <div className="credential-overview-grid">
        <Metric label="已配置" value={value.configuredCount} tone="ok" />
        <Metric label="即将过期（7天内）" value={value.expiringSoonCount} tone="warn" />
        <Metric label="已过期" value={value.expiredCount} tone="failed" />
      </div>
    </section>
  );
}

function Metric(props: { label: string; value: number; tone: "ok" | "warn" | "failed" }) {
  return (
    <div className="credential-overview-metric">
      <span>{props.label}</span>
      <strong className={`credential-number-${props.tone}`}>{props.value}</strong>
    </div>
  );
}
