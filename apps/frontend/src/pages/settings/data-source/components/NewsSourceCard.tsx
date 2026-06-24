import { Button } from "antd";
import { FileTextOutlined, ReloadOutlined } from "@ant-design/icons";
import type { NewsSourceItem, SourceStatus } from "../types";

type NewsSourceCardProps = {
  items: NewsSourceItem[];
  onSyncNow: () => void;
  onViewLog: () => void;
};

export function NewsSourceCard(props: NewsSourceCardProps) {
  return (
    <section className="settings-basic-card settings-basic-mini-card settings-data-source-card">
      <CardHeader title="资讯与新闻源" description="管理个股新闻、行业资讯与研究线索来源" />
      <div className="settings-data-source-row-list">
        {props.items.length > 0 ? (
          props.items.map((item) => (
            <div key={item.label} className="settings-data-source-status-row settings-data-source-three-col-row">
              <span>{item.label}</span>
              <StatusTag status={item.status} />
              <strong>{item.source}</strong>
            </div>
          ))
        ) : (
          <div className="settings-data-source-empty-row">暂无新闻 Provider 状态</div>
        )}
      </div>
      <div className="settings-basic-button-row settings-data-source-button-row">
        <Button className="settings-basic-outline-button" icon={<ReloadOutlined />} onClick={props.onSyncNow}>
          立即同步
        </Button>
        <Button className="settings-basic-outline-button" icon={<FileTextOutlined />} onClick={props.onViewLog}>
          查看日志
        </Button>
      </div>
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

function StatusTag(props: { status: SourceStatus }) {
  const text = props.status === "enabled" ? "已启用" : props.status === "disabled" ? "未启用" : props.status === "failed" ? "异常" : "正常";
  const className = props.status === "failed" ? "settings-data-source-tag-failed" : props.status === "disabled" ? "settings-data-source-tag-muted" : "settings-data-source-tag-ok";
  return <span className={["settings-data-source-tag", className].join(" ")}>{text}</span>;
}
