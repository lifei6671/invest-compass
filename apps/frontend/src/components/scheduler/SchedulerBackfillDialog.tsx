import type { FormEvent } from "react";
import { Button } from "antd";
import type { SchedulerJob } from "../../services/scheduler";

export type SchedulerBackfillFormState = {
  jobId: string;
  dateFrom: string;
  dateTo: string;
  symbols: string;
};

type SchedulerBackfillDialogProps = {
  value: SchedulerBackfillFormState;
  jobs: SchedulerJob[];
  actionPending: string | null;
  onChange: (value: SchedulerBackfillFormState) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

/// SchedulerBackfillDialog 只维护补偿表单的受控展示，范围校验和入队由页面层统一处理。
export function SchedulerBackfillDialog(props: SchedulerBackfillDialogProps) {
  const update = (patch: Partial<SchedulerBackfillFormState>) => {
    props.onChange({ ...props.value, ...patch });
  };
  return (
    <form className="grid gap-3 px-4 py-4 md:grid-cols-[1.2fr_1fr_1fr_1.4fr_auto]" onSubmit={props.onSubmit}>
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">任务</span>
        <select aria-label="任务" className="h-8 rounded border border-slate-300 px-2" value={props.value.jobId} onChange={(event) => update({ jobId: event.target.value })}>
          <option value="">请选择</option>
          {props.jobs.map((job) => (
            <option key={job.id} value={job.id}>
              {job.name || `任务 ${job.id}`}
            </option>
          ))}
        </select>
      </label>
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">开始日期</span>
        <input
          aria-label="开始日期"
          className="h-8 rounded border border-slate-300 px-2"
          type="date"
          value={props.value.dateFrom}
          onChange={(event) => update({ dateFrom: event.target.value })}
        />
      </label>
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">结束日期</span>
        <input
          aria-label="结束日期"
          className="h-8 rounded border border-slate-300 px-2"
          type="date"
          value={props.value.dateTo}
          onChange={(event) => update({ dateTo: event.target.value })}
        />
      </label>
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">Symbol，可选</span>
        <input
          aria-label="Symbol，可选"
          className="h-8 rounded border border-slate-300 px-2"
          placeholder="600000.SH, 000001.SZ"
          value={props.value.symbols}
          onChange={(event) => update({ symbols: event.target.value })}
        />
      </label>
      <Button aria-label="生成补偿" className="self-end" htmlType="submit" loading={props.actionPending === "backfill"}>
        生成补偿
      </Button>
    </form>
  );
}
