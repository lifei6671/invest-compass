import type { DataSourceProvider } from "../types";
import { authTypeText, CredentialStatusTag } from "./CredentialStatusTag";

type CredentialProviderListProps = {
  items: DataSourceProvider[];
  selectedProviderId: string;
  onSelect: (provider: DataSourceProvider) => void;
};

export function CredentialProviderList(props: CredentialProviderListProps) {
  return (
    <section className="credential-card credential-provider-card">
      <h2>Provider 列表</h2>
      <div className="credential-provider-list">
        {props.items.map((item) => {
          const selected = item.id === props.selectedProviderId;
          return (
            <button key={item.id} type="button" className={["credential-provider-row", selected ? "credential-provider-row-active" : ""].join(" ")} onClick={() => props.onSelect(item)}>
              <ProviderIcon type={item.iconType} />
              <span className="credential-provider-main">
                <strong>{item.name}</strong>
                <span>{item.capability}</span>
              </span>
              <span className="credential-provider-meta">
                <CredentialStatusTag status={item.status} />
                <span>{authTypeText(item.authType)}</span>
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}

function ProviderIcon({ type }: { type: string }) {
  if (type === "akshare") {
    return <span className="credential-provider-icon credential-provider-icon-akshare">▲</span>;
  }
  if (type === "sina") {
    return <span className="credential-provider-icon credential-provider-icon-sina">S</span>;
  }
  if (type === "tencent") {
    return <span className="credential-provider-icon credential-provider-icon-tencent">T</span>;
  }
  if (type === "alpha") {
    return <span className="credential-provider-icon credential-provider-icon-alpha">AV</span>;
  }
  if (type === "cls") {
    return <span className="credential-provider-icon credential-provider-icon-cls">C</span>;
  }
  if (type === "xueqiu") {
    return <span className="credential-provider-icon credential-provider-icon-xueqiu">◇</span>;
  }
  if (type === "custom") {
    return <span className="credential-provider-icon credential-provider-icon-custom">&lt;&gt;</span>;
  }
  return <span className="credential-provider-icon credential-provider-icon-eastmoney">↗</span>;
}
