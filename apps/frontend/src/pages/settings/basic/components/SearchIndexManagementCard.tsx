import { Button, Space, Tag } from "antd";
import { ReloadOutlined, SearchOutlined, SyncOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { SearchIndexStatus, SearchRebuildPayload } from "../../../../services/coreClient";

type SearchIndexManagementCardProps = {
  value: SearchIndexStatus | null;
  loading?: boolean;
  rebuildingScope?: SearchRebuildPayload["scope"] | null;
  onRefresh: () => void;
  onRebuild: (scope: SearchRebuildPayload["scope"]) => void;
};

const rebuildButtons: Array<{ scope: SearchRebuildPayload["scope"]; label: string }> = [
  { scope: "all", label: "重建全部索引" },
  { scope: "stock", label: "重建股票索引" },
  { scope: "reports", label: "重建报告索引" },
  { scope: "news", label: "重建新闻索引" },
  { scope: "watchlist_notes", label: "重建备注索引" },
];

export function SearchIndexManagementCard(props: SearchIndexManagementCardProps) {
  const rebuilding = props.rebuildingScope != null;

  return (
    <section className="settings-basic-card settings-basic-mini-card">
      <CardHeader icon={<SearchOutlined />} title="搜索索引" description="查看本地 FTS 状态并触发菜单范围索引重建" />
      <div className="settings-basic-kv-list">
        <KeyValue label="FTS5" value={<StatusTag value={props.value?.fts5_status} />} />
        <KeyValue label="GSE" value={<StatusTag value={props.value?.gse_status} />} />
        <KeyValue label="搜索状态" value={<StatusTag value={props.value?.search_status} />} />
        <KeyValue label="股票索引数量" value={formatCount(props.value?.stock_index_count)} />
        <KeyValue label="报告索引数量" value={formatCount(props.value?.report_index_count)} />
        <KeyValue label="新闻索引数量" value={formatCount(props.value?.news_index_count)} />
        <KeyValue label="自选备注索引数量" value={formatCount(props.value?.watchlist_note_index_count)} />
        <KeyValue label="最后重建时间" value={props.value?.last_rebuild_at || "—"} />
        <KeyValue label="分词器" value={formatTokenizer(props.value)} />
        <KeyValue label="词典版本" value={props.value?.dictionary_hash || "—"} />
        <KeyValue label="股票索引批次" value={props.value?.active_stock_batch_id || "—"} />
        <KeyValue label="文档索引批次" value={props.value?.active_document_batch_id || "—"} />
        <KeyValue label="重建任务" value={props.value?.running_rebuild_task_id || "无运行任务"} />
      </div>
      <Space wrap>
        <Button className="settings-basic-outline-button" icon={<ReloadOutlined />} loading={props.loading} onClick={props.onRefresh}>
          刷新状态
        </Button>
        {rebuildButtons.map((item) => (
          <Button
            key={item.scope}
            className="settings-basic-outline-button"
            icon={<SyncOutlined />}
            loading={props.rebuildingScope === item.scope}
            disabled={rebuilding && props.rebuildingScope !== item.scope}
            onClick={() => props.onRebuild(item.scope)}
          >
            {item.label}
          </Button>
        ))}
      </Space>
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

function KeyValue(props: { label: string; value: ReactNode }) {
  return (
    <div className="settings-basic-kv-row">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}

function StatusTag(props: { value?: string }) {
  const normalized = (props.value || "unknown").trim().toUpperCase();
  const color =
    normalized === "READY" || normalized === "AVAILABLE"
      ? "green"
      : normalized === "BUILDING"
        ? "blue"
        : normalized === "FALLBACK"
          ? "orange"
          : "default";
  return <Tag color={color}>{normalized}</Tag>;
}

function formatCount(value?: number) {
  return typeof value === "number" ? value.toLocaleString("zh-CN") : "—";
}

function formatTokenizer(value: SearchIndexStatus | null) {
  if (!value?.tokenizer_name) {
    return "—";
  }
  return value.tokenizer_version ? `${value.tokenizer_name}@${value.tokenizer_version}` : value.tokenizer_name;
}
