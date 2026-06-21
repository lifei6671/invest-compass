import { Button } from "antd";
import { DeleteOutlined } from "@ant-design/icons";
import type { CacheSnapshot } from "../types";

type LocalCacheSnapshotCardProps = {
  value: CacheSnapshot;
  onCleanCache: () => void;
};

export function LocalCacheSnapshotCard(props: LocalCacheSnapshotCardProps) {
  return (
    <section className="settings-basic-card settings-basic-mini-card settings-data-source-card">
      <CardHeader title="本地缓存与快照" description="数据落地缓存与上下文快照管理" />
      <div className="settings-data-source-kv-list">
        <KeyValue label="行情缓存大小" value={props.value.quoteCacheSize} />
        <KeyValue label="新闻缓存大小" value={props.value.newsCacheSize} />
        <KeyValue label="最近快照时间" value={props.value.latestSnapshotTime} />
        <KeyValue label="快照保留策略" value={props.value.retentionPolicy} />
      </div>
      <Button className="settings-basic-outline-button settings-data-source-single-button" icon={<DeleteOutlined />} onClick={props.onCleanCache}>
        清理缓存
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

function KeyValue(props: { label: string; value: string }) {
  return (
    <div className="settings-data-source-kv-row">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
