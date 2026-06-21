import { AppstoreOutlined, CodeOutlined, DatabaseOutlined, FolderOpenOutlined, InfoCircleOutlined, ProfileOutlined, RocketOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { AppInfoItem } from "../types";

type AppInfoCardProps = {
  items: AppInfoItem[];
};

export function AppInfoCard(props: AppInfoCardProps) {
  return (
    <section className="settings-basic-card settings-about-info-card">
      <CardTitle icon={<InfoCircleOutlined />} title="应用信息" />
      <div className="settings-about-info-list">
        {props.items.map((item) => (
          <div key={item.key} className="settings-about-info-row">
            <span className="settings-about-row-icon">{infoIcon(item.key)}</span>
            <span>{item.label}</span>
            <strong className={item.key === "workspace" || item.key === "logs" ? "settings-about-path-value" : ""}>
              {item.status ? <span className="settings-data-source-dot settings-data-source-dot-ok" /> : null}
              {item.value}
            </strong>
          </div>
        ))}
      </div>
    </section>
  );
}

function CardTitle(props: { icon: ReactNode; title: string }) {
  return (
    <header className="settings-about-card-title">
      <span>{props.icon}</span>
      <h2>{props.title}</h2>
    </header>
  );
}

function infoIcon(key: string) {
  if (key === "frontend") return <CodeOutlined />;
  if (key === "core") return <RocketOutlined />;
  if (key === "database") return <DatabaseOutlined />;
  if (key === "workspace") return <FolderOpenOutlined />;
  if (key === "logs") return <ProfileOutlined />;
  return <AppstoreOutlined />;
}
