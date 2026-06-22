import type { CredentialOperationLog } from "../types";
import { CredentialStatusTag } from "./CredentialStatusTag";

export function CredentialOperationLogCard({ items }: { items: CredentialOperationLog[] }) {
  return (
    <section className="credential-card credential-bottom-card">
      <h2>凭据操作日志</h2>
      <div className="credential-log-list">
        {items.map((item) => (
          <div key={item.id} className="credential-log-row">
            <span>{item.action}</span>
            <CredentialStatusTag status={item.status} />
            <strong>{item.time}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}
