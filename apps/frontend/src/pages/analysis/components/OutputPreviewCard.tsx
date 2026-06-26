import { FullscreenOutlined } from "@ant-design/icons";
import { Button, Empty, Modal, Select } from "antd";
import { useState } from "react";
import type { OutputFormat } from "../types";

type OutputPreviewCardProps = {
  markdown: string;
  plainText: string;
  format: OutputFormat;
  onFormatChange: (format: OutputFormat) => void;
};

export function OutputPreviewCard(props: OutputPreviewCardProps) {
  const [fullscreenOpen, setFullscreenOpen] = useState(false);
  const preview = props.format === "Markdown" ? <MarkdownPreview markdown={props.markdown} /> : <PlainTextPreview text={props.plainText} />;

  return (
    <section className="analysis-card analysis-output-card">
      <header className="analysis-output-header">
        <div className="analysis-output-title">
          <h2 className="analysis-card-title">输出预览</h2>
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
          <Button aria-label="全屏预览" className="analysis-icon-button" icon={<FullscreenOutlined />} onClick={() => setFullscreenOpen(true)} />
        </div>
      </header>
      <div className="analysis-output-body">
        {preview}
      </div>
      <Modal aria-label="全屏预览" title={null} open={fullscreenOpen} footer={null} width={1120} centered destroyOnHidden onCancel={() => setFullscreenOpen(false)}>
        <h2 className="analysis-output-modal-title">全屏预览</h2>
        <div className="analysis-output-modal-body">{preview}</div>
      </Modal>
    </section>
  );
}

function MarkdownPreview(props: { markdown: string }) {
  if (!props.markdown.trim()) {
    return <Empty description="暂无输出内容" />;
  }

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
  if (!props.text.trim()) {
    return <Empty description="暂无输出内容" />;
  }

  return <pre className="analysis-plain-preview">{props.text}</pre>;
}
