import {
  CompressOutlined,
  DownOutlined,
  FullscreenOutlined,
  RedoOutlined,
  SearchOutlined,
  ToolOutlined,
  UndoOutlined,
} from "@ant-design/icons";
import { Button, Input, Select } from "antd";
import type { ReactNode } from "react";
import type { PromptEditorState, PromptTemplateCategoryType } from "../types";

type PromptTemplateEditorProps = {
  value: PromptEditorState;
  templateTypeOptions: Array<{ label: string; value: PromptTemplateCategoryType }>;
  readOnly?: boolean;
  onChange: (value: PromptEditorState) => void;
  onToolAction: (action: string) => void;
  onFormat: () => void;
  onFullscreen: () => void;
};

export function PromptTemplateEditor(props: PromptTemplateEditorProps) {
  const lineNumbers = props.value.promptContent.split("\n").map((_, index) => index + 1);
  const update = <Key extends keyof PromptEditorState>(key: Key, value: PromptEditorState[Key]) => {
    props.onChange({ ...props.value, [key]: value });
  };

  return (
    <section className="prompt-card prompt-editor-card prompt-editor-card-aligned">
      <div className="prompt-editor-form-grid">
        <label className="prompt-editor-field">
          <span>模板名称</span>
          <Input
            value={props.value.templateName}
            disabled={props.readOnly}
            onChange={(event) => update("templateName", event.target.value)}
          />
        </label>
        <label className="prompt-editor-field">
          <span>模板类型</span>
          <Select
            value={props.value.templateType}
            options={props.templateTypeOptions}
            disabled={props.readOnly}
            onChange={(value) => update("templateType", value)}
          />
        </label>
      </div>
      <label className="prompt-editor-field prompt-description-field">
        <span>模板描述</span>
        <Input.TextArea
          autoSize={false}
          maxLength={200}
          value={props.value.templateDescription}
          disabled={props.readOnly}
          onChange={(event) => update("templateDescription", event.target.value)}
        />
        <em>{props.value.templateDescription.length}/200</em>
      </label>
      <h3 className="prompt-editor-title">Prompt 内容</h3>
      <div className="prompt-editor-shell prompt-editor-shell-aligned">
        <div className="prompt-editor-toolbar">
          <div className="prompt-editor-tool-group">
            <ToolButton label="撤销" icon={<UndoOutlined />} onClick={() => props.onToolAction("撤销待接入")} />
            <ToolButton label="重做" icon={<RedoOutlined />} onClick={() => props.onToolAction("重做待接入")} />
            <ToolButton label="搜索" icon={<SearchOutlined />} onClick={() => props.onToolAction("搜索待接入")} />
            <ToolButton label="插入变量" icon={<CompressOutlined />} onClick={() => props.onToolAction("插入变量待接入")} />
            <ToolButton label="格式检查" icon={<DownOutlined />} onClick={() => props.onToolAction("格式检查待接入")} />
            <ToolButton label="变量符号" text="{}" onClick={() => props.onToolAction("变量符号待接入")} />
          </div>
          <div className="prompt-editor-tool-group">
            <Button size="small" className="prompt-format-button" onClick={props.onFormat}>
              格式化
            </Button>
            <Button aria-label="全屏编辑" size="small" className="prompt-editor-icon-only" icon={<FullscreenOutlined />} onClick={props.onFullscreen} />
          </div>
        </div>
        <div className="prompt-editor-body prompt-editor-body-scroll-safe prompt-editor-body-scrollable prompt-editor-body-contained">
          <div className="prompt-line-numbers prompt-line-numbers-contained" aria-hidden="true">
            {lineNumbers.map((line) => (
              <span key={line}>{line}</span>
            ))}
          </div>
          <textarea
            aria-label="Prompt 内容"
            className="prompt-editor-textarea prompt-editor-textarea-wrap prompt-editor-textarea-fit prompt-editor-textarea-bottom-safe prompt-editor-textarea-scrollable"
            value={props.value.promptContent}
            readOnly={props.readOnly}
            spellCheck={false}
            wrap="soft"
            onChange={(event) => update("promptContent", event.target.value)}
          />
        </div>
      </div>
    </section>
  );
}

function ToolButton(props: { label: string; icon?: ReactNode; text?: string; onClick: () => void }) {
  return (
    <Button aria-label={props.label} size="small" className="prompt-editor-icon-only" icon={props.icon} onClick={props.onClick}>
      {props.text}
    </Button>
  );
}
