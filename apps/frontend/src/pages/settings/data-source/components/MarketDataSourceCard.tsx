import { Button } from "antd";
import { LinkOutlined, SettingOutlined } from "@ant-design/icons";
import type { MarketDataSourceItem, SourceStatus } from "../types";

type MarketDataSourceCardProps = {
  items: MarketDataSourceItem[];
  onTestConnection: () => void;
  onEditConfig: () => void;
};

export function MarketDataSourceCard(props: MarketDataSourceCardProps) {
  return (
    <section className="settings-basic-card settings-basic-mini-card settings-data-source-card">
      <CardHeader title="行情数据源" description="管理搜索、个股、K 线与估值数据来源" />
      <div className="settings-data-source-row-list">
        {props.items.map((item) => (
          <div key={item.label} className="settings-data-source-status-row settings-data-source-three-col-row">
            <span>{item.label}</span>
            <StatusTag status={item.status} />
            <strong>{item.source}</strong>
          </div>
        ))}
      </div>
      <div className="settings-basic-button-row settings-data-source-button-row">
        <Button className="settings-basic-outline-button" icon={<LinkOutlined />} onClick={props.onTestConnection}>
          测试连接
        </Button>
        <Button className="settings-basic-outline-button" icon={<SettingOutlined />} onClick={props.onEditConfig}>
          编辑配置
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
  const text = props.status === "disabled" ? "未启用" : props.status === "enabled" ? "已启用" : props.status === "failed" ? "异常" : "正常";
  return <span className={["settings-data-source-tag", props.status === "disabled" ? "settings-data-source-tag-muted" : "settings-data-source-tag-ok"].join(" ")}>{text}</span>;
}
