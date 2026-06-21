import { BookOutlined, ExportOutlined } from "@ant-design/icons";

type UserManualCardProps = {
  description: string;
  actionText: string;
  onAction: () => void;
};

export function UserManualCard(props: UserManualCardProps) {
  return (
    <section className="settings-basic-card settings-about-resource-card">
      <header>
        <BookOutlined />
        <h3>用户手册</h3>
      </header>
      <p>{props.description}</p>
      <button type="button" onClick={props.onAction}>
        {props.actionText}
        <ExportOutlined />
      </button>
    </section>
  );
}
