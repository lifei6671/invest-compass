import { FullscreenOutlined } from "@ant-design/icons";
import { Button, Select } from "antd";
import type { OutputFormat } from "../types";

type OutputPreviewCardProps = {
  markdown: string;
  plainText: string;
  format: OutputFormat;
  onFormatChange: (format: OutputFormat) => void;
  onFullscreen: () => void;
};

export function OutputPreviewCard(props: OutputPreviewCardProps) {
  return (
    <section className="analysis-card analysis-output-card">
      <header className="analysis-output-header">
        <div className="analysis-output-title">
          <h2 className="analysis-card-title">输出预览</h2>
          <span>（示例）</span>
        </div>
        <div className="analysis-output-tools">
          <Select
            className="analysis-output-format"
            value={props.format}
            options={[
              { value: "Markdown", label: "Markdown" },
              { value: "纯文本", label: "纯文本" },
            ]}
            onChange={props.onFormatChange}
          />
          <Button aria-label="全屏预览" className="analysis-icon-button" icon={<FullscreenOutlined />} onClick={props.onFullscreen} />
        </div>
      </header>
      <div className="analysis-output-body">
        {props.format === "Markdown" ? <MarkdownPreview markdown={props.markdown} /> : <PlainTextPreview text={props.plainText} />}
      </div>
    </section>
  );
}

function MarkdownPreview(props: { markdown: string }) {
  const lines = props.markdown.split("\n");
  return (
    <div className="analysis-markdown-preview">
      {lines.map((line, index) => {
        if (!line.trim()) {
          return <div key={index} className="analysis-md-gap" />;
        }
        if (line.startsWith("# ")) {
          return <h1 key={index}>{line.replace(/^#\s+/, "")}</h1>;
        }
        if (line.startsWith("## ")) {
          return <h2 key={index}>{line.replace(/^##\s+/, "")}</h2>;
        }
        if (line.startsWith("- ")) {
          return <p key={index} className="analysis-md-bullet">{line.replace(/^- /, "")}</p>;
        }
        return <p key={index}>{line}</p>;
      })}
    </div>
  );
}

function PlainTextPreview(props: { text: string }) {
  return <pre className="analysis-plain-preview">{props.text}</pre>;
}
