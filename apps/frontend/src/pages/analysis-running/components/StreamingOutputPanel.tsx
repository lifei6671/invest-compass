import { LoadingOutlined } from "@ant-design/icons";
import { Button, Empty, Switch } from "antd";

type StreamingOutputPanelProps = {
  autoScroll: boolean;
  markdown: string;
  onAutoScrollChange: (checked: boolean) => void;
  onClear: () => void;
};

export function StreamingOutputPanel(props: StreamingOutputPanelProps) {
  const hasContent = props.markdown.trim().length > 0;

  return (
    <section className="analysis-running-card analysis-running-stream-card">
      <div className="analysis-running-card-header">
        <h2 className="analysis-running-card-title">
          流式输出 {hasContent ? <span>（正在生成中...）</span> : null}
        </h2>
        <div className="analysis-running-stream-tools">
          <span>自动滚动</span>
          <Switch size="small" checked={props.autoScroll} onChange={props.onAutoScrollChange} />
          <span className="analysis-running-tool-separator" />
          <Button size="small" className="analysis-running-small-button" onClick={props.onClear}>
            清空
          </Button>
        </div>
      </div>
      <div className="analysis-running-stream-body">
        {hasContent ? <MarkdownPreview markdown={props.markdown} /> : <Empty description="暂无流式输出" />}
      </div>
      {hasContent ? (
        <div className="analysis-running-generating-note">
          <LoadingOutlined />
          <span>内容持续生成中...</span>
        </div>
      ) : null}
    </section>
  );
}

function MarkdownPreview({ markdown }: { markdown: string }) {
  return (
    <div className="analysis-running-markdown">
      {markdown.split("\n").map((line, index) => {
        if (line.startsWith("# ")) {
          return <h1 key={line + index}>{line.slice(2)}</h1>;
        }
        if (line.startsWith("## ")) {
          return <h2 key={line + index}>{line.slice(3)}</h2>;
        }
        if (line.startsWith("- ")) {
          return (
            <p key={line + index} className="analysis-running-md-bullet">
              {renderCursor(line.slice(2))}
            </p>
          );
        }
        if (!line.trim()) {
          return <div key={`gap-${index}`} className="analysis-running-md-gap" />;
        }
        return <p key={line + index}>{renderCursor(line)}</p>;
      })}
    </div>
  );
}

function renderCursor(text: string) {
  if (!text.includes("▌")) {
    return text;
  }
  const parts = text.split("▌");
  return (
    <>
      {parts[0]}
      <span className="analysis-running-cursor">▌</span>
      {parts.slice(1).join("▌")}
    </>
  );
}
