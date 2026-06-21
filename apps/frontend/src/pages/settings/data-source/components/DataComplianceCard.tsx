import { InfoCircleOutlined } from "@ant-design/icons";
import { Button } from "antd";

type DataComplianceCardProps = {
  onViewDescription: () => void;
};

const complianceNotes = [
  "数据仅用于本地研究与分析展示",
  "不同数据源更新时间可能存在差异",
  "AI 分析会引用行情与资讯快照构建上下文",
  "请以公开披露信息和实际数据源为准",
];

export function DataComplianceCard(props: DataComplianceCardProps) {
  return (
    <section className="settings-basic-card settings-basic-mini-card settings-data-source-card">
      <CardHeader title="数据合规与说明" description="数据使用边界与研究提示" />
      <ul className="settings-data-source-bullet-list">
        {complianceNotes.map((note) => (
          <li key={note}>{note}</li>
        ))}
      </ul>
      <Button className="settings-basic-outline-button settings-data-source-single-button" icon={<InfoCircleOutlined />} onClick={props.onViewDescription}>
        查看数据说明
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
