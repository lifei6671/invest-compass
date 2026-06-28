import { LoadingOutlined } from "@ant-design/icons";
import { Empty, Switch } from "antd";
import { useEffect, useRef } from "react";

type StreamingOutputPanelProps = {
  autoScroll: boolean;
  generating: boolean;
  markdown: string;
  onAutoScrollChange: (checked: boolean) => void;
};

export function StreamingOutputPanel(props: StreamingOutputPanelProps) {
  const hasContent = props.markdown.trim().length > 0;
  const bodyRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const body = bodyRef.current;
    if (!body || !props.autoScroll || !hasContent) {
      return;
    }
    if (typeof body.scrollTo === "function") {
      body.scrollTo({ top: body.scrollHeight, behavior: "smooth" });
      return;
    }
    body.scrollTop = body.scrollHeight;
  }, [props.autoScroll, props.markdown, hasContent]);

  return (
    <section className="analysis-running-card analysis-running-stream-card">
      <div className="analysis-running-card-header">
        <h2 className="analysis-running-card-title">
          流式输出 {hasContent && props.generating ? <span>（正在生成中...）</span> : null}
        </h2>
        <div className="analysis-running-stream-tools">
          <span>自动滚动</span>
          <Switch size="small" checked={props.autoScroll} onChange={props.onAutoScrollChange} />
        </div>
      </div>
      <div ref={bodyRef} className="analysis-running-stream-body">
        {hasContent ? <MarkdownPreview markdown={props.markdown} /> : <Empty description="暂无流式输出" />}
      </div>
      {hasContent && props.generating ? (
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
