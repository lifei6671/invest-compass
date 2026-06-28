import { CopyOutlined, FileTextOutlined, InfoCircleOutlined, StopOutlined } from "@ant-design/icons";
import { Button } from "antd";

type RunningActionBarProps = {
  generating: boolean;
  reportID?: number | null;
  onStop: () => void;
  onCopy: () => void;
  onViewReport: () => void;
};

export function RunningActionBar(props: RunningActionBarProps) {
  return (
    <div className="analysis-running-action-bar">
      <Button danger className="analysis-running-danger-button" icon={<StopOutlined />} disabled={!props.generating} onClick={props.onStop}>
        停止生成
      </Button>
      <Button className="analysis-running-action-button analysis-running-copy-button" icon={<CopyOutlined />} onClick={props.onCopy}>
        复制当前内容
      </Button>
      {props.reportID ? (
        <Button className="analysis-running-action-button" icon={<FileTextOutlined />} onClick={props.onViewReport}>
          查看报告
        </Button>
      ) : null}
      <div className="analysis-running-action-note">
        <InfoCircleOutlined />
        <span>任务完成后可在报告历史中查看完整内容。</span>
      </div>
    </div>
  );
}
