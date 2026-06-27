import { CopyOutlined, DownloadOutlined, PlayCircleOutlined, SaveOutlined, StopOutlined } from "@ant-design/icons";
import { Button } from "antd";

type AnalysisActionBarProps = {
  generating: boolean;
  startLoading?: boolean;
  startDisabled?: boolean;
  stopDisabled?: boolean;
  saveDisabled?: boolean;
  copyDisabled?: boolean;
  exportDisabled?: boolean;
  onStart: () => void;
  onStop: () => void;
  onSave: () => void;
  onCopy: () => void;
  onExport: () => void;
};

export function AnalysisActionBar(props: AnalysisActionBarProps) {
  return (
    <div className="analysis-action-bar">
      <Button type="primary" className="analysis-start-button" icon={<PlayCircleOutlined />} loading={props.startLoading} disabled={props.startDisabled} onClick={props.onStart}>
        {props.startLoading ? "创建任务中..." : "开始分析"}
      </Button>
      <Button className="analysis-action-button" icon={<StopOutlined />} disabled={props.stopDisabled ?? !props.generating} onClick={props.onStop}>
        停止生成
      </Button>
      <Button className="analysis-action-button" icon={<SaveOutlined />} disabled={props.saveDisabled} onClick={props.onSave}>
        保存报告
      </Button>
      <Button className="analysis-action-button analysis-copy-button" icon={<CopyOutlined />} disabled={props.copyDisabled} onClick={props.onCopy}>
        复制 Markdown
      </Button>
      <Button className="analysis-action-button analysis-export-button" icon={<DownloadOutlined />} disabled={props.exportDisabled} onClick={props.onExport}>
        导出 Markdown
      </Button>
    </div>
  );
}
