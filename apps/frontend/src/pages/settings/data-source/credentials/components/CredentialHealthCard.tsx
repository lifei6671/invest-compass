import type { CredentialHealthItem } from "../types";
import { CredentialStatusTag } from "./CredentialStatusTag";

export function CredentialHealthCard({ items }: { items: CredentialHealthItem[] }) {
  return (
    <section className="credential-card credential-bottom-card">
      <h2>调用限制与健康状态</h2>
      <div className="credential-health-list">
        {items.map((item) => (
          <div key={item.name} className="credential-health-row">
            <span>{item.name}</span>
            <CredentialStatusTag status={item.status} />
            <strong>{item.rateLimitText}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}
