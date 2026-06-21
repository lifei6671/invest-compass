import { Button, Input } from "antd";
import { FolderOutlined } from "@ant-design/icons";
import type { ReactNode } from "react";
import type { WorkspaceSettingsState } from "../types";

type WorkspaceSettingsCardProps = {
  value: WorkspaceSettingsState;
  onSelectDirectory: () => void;
  onOpenDirectory: () => void;
};

export function WorkspaceSettingsCard(props: WorkspaceSettingsCardProps) {
  return (
    <section className="settings-basic-card settings-basic-mini-card">
      <CardHeader icon={<FolderOutlined />} title="工作区设置" description="管理应用的工作区路径与数据存储位置" />
      <div className="settings-basic-card-body">
        <label className="settings-basic-small-label">当前工作区路径</label>
        <Input className="settings-basic-input" value={props.value.workspacePath} readOnly />
        <div className="settings-basic-button-row">
          <Button type="primary" className="settings-basic-primary-button" onClick={props.onSelectDirectory}>
            选择目录
          </Button>
          <Button className="settings-basic-secondary-button" onClick={props.onOpenDirectory}>
            打开目录
          </Button>
        </div>
      </div>
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
