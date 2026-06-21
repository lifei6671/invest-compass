import { LoadingOutlined } from "@ant-design/icons";
import type { ConnectionStatus } from "../types";

export function ModelStatusTag(props: { status: ConnectionStatus }) {
  if (props.status === "testing") {
    return (
      <span className="inline-flex items-center gap-1 text-[12px] text-[#1677ff]">
        <LoadingOutlined className="text-[10px]" />
        测试中
      </span>
    );
  }
  const meta =
    props.status === "normal"
      ? { color: "#16a34a", label: "正常" }
      : props.status === "untested"
        ? { color: "#f97316", label: "未测试" }
        : { color: "#ff4d4f", label: "连接失败" };
  return (
    <span className="inline-flex items-center gap-1 text-[12px] text-[#374151]">
      <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: meta.color }} />
      <span style={{ color: meta.color }}>{meta.label}</span>
    </span>
  );
}
