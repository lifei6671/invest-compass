import { useState } from "react";
import { App as AntApp, Tooltip } from "antd";
import { CopyOutlined, DownOutlined, UpOutlined } from "@ant-design/icons";

type RawJsonPanelProps = {
  value: Record<string, unknown>;
};

export function RawJsonPanel({ value }: RawJsonPanelProps) {
  const { message } = AntApp.useApp();
  const [expanded, setExpanded] = useState(true);
  const jsonText = JSON.stringify(value, null, 2);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(jsonText);
    message.success("JSON 已复制");
  };

  return (
    <section className="task-log-raw-panel">
      <header>
        <button type="button" onClick={() => setExpanded((current) => !current)}>
          原始日志 / JSON 详情
          {expanded ? <UpOutlined /> : <DownOutlined />}
        </button>
        <Tooltip title="复制 JSON">
          <button type="button" onClick={handleCopy} aria-label="复制 JSON">
            <CopyOutlined />
          </button>
        </Tooltip>
      </header>
      {expanded ? <pre>{jsonText}</pre> : null}
    </section>
  );
}
