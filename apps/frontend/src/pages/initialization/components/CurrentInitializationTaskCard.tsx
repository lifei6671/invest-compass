import { InitializationLogList } from "./InitializationLogList";
import type { InitializationLogItem, InitializationTaskDetail } from "../types";

type CurrentInitializationTaskCardProps = {
  detail: InitializationTaskDetail;
  logs: InitializationLogItem[];
};

export function CurrentInitializationTaskCard(props: CurrentInitializationTaskCardProps) {
  return (
    <section className="overflow-hidden rounded-[10px] border border-[#e5eaf3] bg-white">
      <h2 className="border-b border-[#edf1f7] px-5 py-3 text-[15px] font-semibold text-[#111827]">当前任务详情</h2>
      <div className="grid grid-cols-2 border-b border-[#edf1f7]">
        <TaskCell label="Task ID" value={props.detail.taskId} />
        <TaskCell label="已耗时" value={props.detail.elapsed} />
        <TaskCell label="当前阶段" value={props.detail.currentStage} />
        <TaskCell label="预计剩余" value={props.detail.remaining} />
      </div>
      <InitializationLogList logs={props.logs} />
    </section>
  );
}

function TaskCell(props: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[74px_1fr] items-center gap-3 border-b border-r border-[#edf1f7] px-4 py-[14px] text-[13px] even:border-r-0 [&:nth-last-child(-n+2)]:border-b-0">
      <span className="text-[12px] font-semibold text-[#64748b]">{props.label}</span>
      <span className="min-w-0 truncate text-[#111827]" title={props.value}>
        {props.value}
      </span>
    </div>
  );
}
