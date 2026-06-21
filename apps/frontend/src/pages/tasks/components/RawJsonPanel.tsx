import { useState } from "react";
import { App as AntApp, Tooltip } from "antd";
import { CopyOutlined, DownOutlined, UpOutlined } from "@ant-design/icons";

type RawJsonPanelProps = {
  value: Record<string, unknown> | null;
  loading?: boolean;
  emptyText?: string;
};

export function RawJsonPanel({
  value,
  loading = false,
  emptyText = "请选择一条日志查看 JSON 详情",
}: RawJsonPanelProps) {
  const { message } = AntApp.useApp();
  const [expanded, setExpanded] = useState(true);
  const jsonText = value ? JSON.stringify(value, null, 2) : emptyText;

  const handleCopy = async () => {
    if (!value || loading) {
      return;
    }
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
          <button type="button" onClick={handleCopy} aria-label="复制 JSON" disabled={!value || loading}>
            <CopyOutlined />
          </button>
        </Tooltip>
      </header>
      {expanded ? <pre>{loading ? "正在加载 JSON 详情..." : jsonText}</pre> : null}
    </section>
  );
}
