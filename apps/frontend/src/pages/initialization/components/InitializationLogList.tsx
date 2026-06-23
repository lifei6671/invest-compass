import type { InitializationLogItem, InitializationLogStatus } from "../types";

type InitializationLogListProps = {
  logs: InitializationLogItem[];
};

export function InitializationLogList(props: InitializationLogListProps) {
  return (
    <div className="h-[210px] overflow-y-auto px-[18px] py-3">
      {props.logs.map((item) => (
        <div key={item.id} className="grid min-h-8 grid-cols-[86px_12px_1fr] items-center gap-3 text-[13px] leading-8 text-[#374151]">
          <span>[{item.time}]</span>
          <span aria-hidden="true" className={["h-2 w-2 rounded-full", logStatusClassName(item.status)].join(" ")} />
          <span>{item.message}</span>
        </div>
      ))}
    </div>
  );
}

function logStatusClassName(status: InitializationLogStatus) {
  if (status === "success") {
    return "bg-[#16a34a]";
  }
  if (status === "running") {
    return "bg-[#1677ff]";
  }
  if (status === "error") {
    return "bg-[#ff4d4f]";
  }
  return "bg-[#9ca3af]";
}
