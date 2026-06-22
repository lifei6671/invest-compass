import { CredentialStatusTag } from "./CredentialStatusTag";

export function CredentialSecurityCard() {
  return (
    <section className="credential-card credential-security-card">
      <h2>安全与存储说明</h2>
      <ul className="credential-security-list">
        <li>凭据保存在本地安全存储</li>
        <li>SQLite 仅保存凭据引用与脱敏标记</li>
        <li>日志导出会自动脱敏 Authorization / Cookie</li>
        <li>未配置凭据的数据源不会返回假数据</li>
      </ul>
      <div className="credential-security-status">
        <div>
          <span>本地 Vault：</span>
          <CredentialStatusTag status="normal" />
        </div>
        <div>
          <span>引用状态：</span>
          <CredentialStatusTag status="success" text="已绑定" />
        </div>
        <div>
          <span>最近更新：</span>
          <strong>2025-05-20 15:28:41</strong>
        </div>
      </div>
    </section>
  );
}
