import { ReloadOutlined } from "@ant-design/icons";
import { Button } from "antd";
import type { HealthStatusItem } from "../types";

type DataSourceHealthCardProps = {
  items: HealthStatusItem[];
  onRefresh: () => void;
};

export function DataSourceHealthCard(props: DataSourceHealthCardProps) {
  return (
    <section className="settings-basic-card settings-basic-mini-card settings-data-source-card">
      <CardHeader title="数据源状态摘要" description="本次启动的关键连接检查结果" />
      <div className="settings-data-source-health-list">
        {props.items.map((item) => (
          <div key={item.label} className="settings-data-source-health-row">
            <span className={["settings-data-source-dot", dotClassName(item.status)].join(" ")} />
            <span>{item.label}</span>
            <strong>{item.statusText}</strong>
          </div>
        ))}
      </div>
      <Button className="settings-basic-outline-button settings-data-source-single-button" icon={<ReloadOutlined />} onClick={props.onRefresh}>
        重新检测
      </Button>
    </section>
  );
}

function CardHeader(props: { title: string; description: string }) {
  return (
    <header className="settings-data-source-card-header">
      <h3>{props.title}</h3>
      <p>{props.description}</p>
    </header>
  );
}

function dotClassName(status: HealthStatusItem["status"]) {
  if (status === "disabled") {
    return "settings-data-source-dot-muted";
  }
  if (status === "failed") {
    return "settings-data-source-dot-failed";
  }
  return "settings-data-source-dot-ok";
}
