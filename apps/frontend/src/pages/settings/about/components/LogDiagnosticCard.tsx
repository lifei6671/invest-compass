import { CodeOutlined, ExportOutlined } from "@ant-design/icons";

type LogDiagnosticCardProps = {
  description: string;
  actionText: string;
  onAction: () => void;
};

export function LogDiagnosticCard(props: LogDiagnosticCardProps) {
  return (
    <section className="settings-basic-card settings-about-resource-card">
      <header>
        <CodeOutlined />
        <h3>日志与诊断</h3>
      </header>
      <p>{props.description}</p>
      <button type="button" onClick={props.onAction}>
        {props.actionText}
        <ExportOutlined />
      </button>
    </section>
  );
}
