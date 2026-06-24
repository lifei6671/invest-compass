import { ExportOutlined, InfoCircleFilled, UpCircleFilled } from "@ant-design/icons";
import { Button } from "antd";
import type { ReactNode } from "react";
import type { UpdateInfo } from "../types";

type CheckUpdateCardProps = {
  value: UpdateInfo;
  checking: boolean;
  onCheck: () => void;
  onViewReleaseNote: () => void;
};

export function CheckUpdateCard(props: CheckUpdateCardProps) {
  return (
    <section className="settings-basic-card settings-about-status-card">
      <CardTitle />
      <div className="settings-about-kv-list">
        <KeyValue label="当前版本" value={props.value.currentVersion} />
        <KeyValue
          label="更新状态"
          value={
            <span className="settings-about-latest-status">
              <InfoCircleFilled />
              检查更新待接入
            </span>
          }
        />
        <KeyValue label="最新版本" value={props.value.latestVersion} />
        <KeyValue label="发布日期" value={props.value.releaseDate} />
      </div>
      <div className="settings-about-action-row">
        <Button type="primary" className="settings-about-primary-button" loading={props.checking} onClick={props.onCheck}>
          检查更新
        </Button>
        <Button className="settings-basic-outline-button settings-about-outline-button" icon={<ExportOutlined />} iconPlacement="end" onClick={props.onViewReleaseNote}>
          查看发布说明
        </Button>
      </div>
    </section>
  );
}

function CardTitle() {
  return (
    <header className="settings-about-card-title">
      <span>
        <UpCircleFilled />
      </span>
      <h2>检查更新</h2>
    </header>
  );
}

function KeyValue(props: { label: string; value: ReactNode }) {
  return (
    <div className="settings-about-kv-row">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}
