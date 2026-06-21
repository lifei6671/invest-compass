import { Button } from "antd";
import { DatabaseOutlined, DeleteOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { CacheSummary } from "../types";

type CacheManagementCardProps = {
  value: CacheSummary;
  onCleanCache: () => void;
};

export function CacheManagementCard(props: CacheManagementCardProps) {
  return (
    <section className="settings-basic-card settings-basic-mini-card">
      <CardHeader icon={<DatabaseOutlined />} title="缓存管理" description="管理本地缓存与临时文件" />
      <div className="settings-basic-kv-list">
        <KeyValue label="当前缓存大小" value={props.value.totalSize} />
        <KeyValue label="临时文件大小" value={props.value.tempSize} />
        <div>
          <div className="settings-basic-small-label">缓存目录</div>
          <div className="settings-basic-path-text" title={props.value.cacheDir}>
            {props.value.cacheDir}
          </div>
        </div>
      </div>
      <Button className="settings-basic-outline-button" icon={<DeleteOutlined />} onClick={props.onCleanCache}>
        清理缓存
      </Button>
    </section>
  );
}

function CardHeader(props: { icon: ReactNode; title: string; description: string }) {
  return (
    <header className="settings-basic-mini-header">
      <span className="settings-basic-mini-icon">{props.icon}</span>
      <div>
        <h3>{props.title}</h3>
        <p>{props.description}</p>
      </div>
    </header>
  );
}

function KeyValue(props: { label: string; value: string }) {
  return (
    <div className="settings-basic-kv-row">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
