import { FullscreenOutlined } from "@ant-design/icons";
import { Button, Select } from "antd";
import { promptPreviewMarkdown } from "../mock";
import type { PromptEditorState } from "../types";

type OutputPreviewPanelProps = {
  format: PromptEditorState["previewFormat"];
  onFormatChange: (format: PromptEditorState["previewFormat"]) => void;
  onFullscreen: () => void;
};

export function OutputPreviewPanel(props: OutputPreviewPanelProps) {
  return (
    <section className="prompt-card prompt-preview-panel">
      <div className="prompt-card-header prompt-compact-header">
        <h2>
          输出预览 <span>（示例）</span>
        </h2>
        <div className="prompt-preview-tools">
          <Select
            className="prompt-preview-select"
            value={props.format}
            options={[
              { label: "Markdown", value: "Markdown" },
              { label: "纯文本", value: "纯文本" },
            ]}
            onChange={props.onFormatChange}
          />
          <Button aria-label="全屏预览" className="prompt-editor-icon-only" icon={<FullscreenOutlined />} onClick={props.onFullscreen} />
        </div>
      </div>
      <div className="prompt-preview-body">
        {props.format === "Markdown" ? <MarkdownPreview markdown={promptPreviewMarkdown} /> : <pre>{plainTextFromMarkdown(promptPreviewMarkdown)}</pre>}
      </div>
    </section>
  );
}

function MarkdownPreview({ markdown }: { markdown: string }) {
  return (
    <div className="prompt-markdown-preview">
      {markdown.split("\n").map((line, index) => {
        if (line.startsWith("# ")) {
          return <h1 key={line + index}>{line.slice(2)}</h1>;
        }
        if (line.startsWith("## ")) {
          return <h2 key={line + index}>{line.slice(3)}</h2>;
        }
        if (line.startsWith("- ")) {
          return (
            <p key={line + index} className="prompt-md-bullet">
              {line.slice(2)}
            </p>
          );
        }
        if (!line.trim()) {
          return <div key={`gap-${index}`} className="prompt-md-gap" />;
        }
        return <p key={line + index}>{line}</p>;
      })}
    </div>
  );
}

function plainTextFromMarkdown(markdown: string) {
  return markdown.replace(/^#{1,2}\s*/gm, "").replace(/^- /gm, "");
}
