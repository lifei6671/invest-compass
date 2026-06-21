import { CopyOutlined, DownloadOutlined, PlayCircleOutlined, SaveOutlined, StopOutlined } from "@ant-design/icons";
import { Button } from "antd";

type AnalysisActionBarProps = {
  generating: boolean;
  onStart: () => void;
  onStop: () => void;
  onSave: () => void;
  onCopy: () => void;
  onExport: () => void;
};

export function AnalysisActionBar(props: AnalysisActionBarProps) {
  return (
    <div className="analysis-action-bar">
      <Button type="primary" className="analysis-start-button" icon={<PlayCircleOutlined />} onClick={props.onStart}>
        开始分析
      </Button>
      <Button className="analysis-action-button" icon={<StopOutlined />} disabled={!props.generating} onClick={props.onStop}>
        停止生成
      </Button>
      <Button className="analysis-action-button" icon={<SaveOutlined />} onClick={props.onSave}>
        保存报告
      </Button>
      <Button className="analysis-action-button analysis-copy-button" icon={<CopyOutlined />} onClick={props.onCopy}>
        复制 Markdown
      </Button>
      <Button className="analysis-action-button analysis-export-button" icon={<DownloadOutlined />} onClick={props.onExport}>
        导出 Markdown
      </Button>
    </div>
  );
}
