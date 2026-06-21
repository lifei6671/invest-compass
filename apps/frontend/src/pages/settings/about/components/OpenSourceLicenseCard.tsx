import { ExportOutlined, FileProtectOutlined } from "@ant-design/icons";

type OpenSourceLicenseCardProps = {
  description: string;
  actionText: string;
  onAction: () => void;
};

export function OpenSourceLicenseCard(props: OpenSourceLicenseCardProps) {
  return (
    <section className="settings-basic-card settings-about-resource-card">
      <header>
        <FileProtectOutlined />
        <h3>开源许可证</h3>
      </header>
      <p>{props.description}</p>
      <button type="button" onClick={props.onAction}>
        {props.actionText}
        <ExportOutlined />
      </button>
    </section>
  );
}
