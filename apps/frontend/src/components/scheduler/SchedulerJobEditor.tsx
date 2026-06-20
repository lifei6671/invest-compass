import type { FormEvent } from "react";
import { Button } from "antd";
import type { SchedulerJobType } from "../../services/scheduler";

export type SchedulerJobFormState = {
  id: number;
  name: string;
  cronType: string;
  cronExpr: string;
  enabled: boolean;
  market: string;
  timezone: string;
  tradeWindow: string;
  symbols: string;
  paramsJSON: string;
  catchupEnabled: boolean;
  catchupMaxDays: string;
  timeoutSeconds: string;
};

type SchedulerJobEditorProps = {
  value: SchedulerJobFormState;
  types: SchedulerJobType[];
  actionPending: string | null;
  onChange: (value: SchedulerJobFormState) => void;
  onSelectType: (cronType: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onReset: () => void;
};

/// SchedulerJobEditor 是调度任务的受控编辑表单；校验和保存由页面层保持单一业务入口。
export function SchedulerJobEditor(props: SchedulerJobEditorProps) {
  const update = (patch: Partial<SchedulerJobFormState>) => {
    props.onChange({ ...props.value, ...patch });
  };
  return (
    <form className="grid gap-3 px-4 py-4 md:grid-cols-4" onSubmit={props.onSubmit}>
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">名称</span>
        <input
          aria-label="名称"
          className="h-8 rounded border border-slate-300 px-2"
          value={props.value.name}
          onChange={(event) => update({ name: event.target.value })}
        />
      </label>
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">类型</span>
        <select
          aria-label="类型"
          className="h-8 rounded border border-slate-300 px-2"
          value={props.value.cronType}
          onChange={(event) => props.onSelectType(event.target.value)}
        >
          {props.types.map((jobType) => (
            <option key={jobType.cron_type} value={jobType.cron_type}>
              {jobType.label || jobType.cron_type}
            </option>
          ))}
        </select>
      </label>
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">Cron</span>
        <input
          aria-label="Cron"
          className="h-8 rounded border border-slate-300 px-2"
          value={props.value.cronExpr}
          onChange={(event) => update({ cronExpr: event.target.value })}
        />
      </label>
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">交易窗口</span>
        <input
          aria-label="交易窗口"
          className="h-8 rounded border border-slate-300 px-2"
          placeholder="09:30-15:00"
          value={props.value.tradeWindow}
          onChange={(event) => update({ tradeWindow: event.target.value })}
        />
      </label>
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">市场</span>
        <input
          aria-label="市场"
          className="h-8 rounded border border-slate-300 px-2"
          value={props.value.market}
          onChange={(event) => update({ market: event.target.value })}
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
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">补偿天数</span>
        <input
          aria-label="补偿天数"
          className="h-8 rounded border border-slate-300 px-2"
          inputMode="numeric"
          value={props.value.catchupMaxDays}
          onChange={(event) => update({ catchupMaxDays: event.target.value })}
        />
      </label>
      <label className="flex flex-col gap-1 text-sm">
        <span className="text-xs text-slate-500">超时秒数</span>
        <input
          aria-label="超时秒数"
          className="h-8 rounded border border-slate-300 px-2"
          inputMode="numeric"
          value={props.value.timeoutSeconds}
          onChange={(event) => update({ timeoutSeconds: event.target.value })}
        />
      </label>
      <label className="flex items-center gap-2 text-sm">
        <input
          aria-label="启用任务"
          type="checkbox"
          checked={props.value.enabled}
          onChange={(event) => update({ enabled: event.target.checked })}
        />
        启用任务
      </label>
      <label className="flex items-center gap-2 text-sm">
        <input
          aria-label="启用补偿"
          type="checkbox"
          checked={props.value.catchupEnabled}
          onChange={(event) => update({ catchupEnabled: event.target.checked })}
        />
        启用补偿
      </label>
      <label className="flex flex-col gap-1 text-sm md:col-span-2">
        <span className="text-xs text-slate-500">参数 JSON</span>
        <textarea
          aria-label="参数 JSON"
          className="min-h-16 rounded border border-slate-300 px-2 py-1 font-mono text-xs"
          value={props.value.paramsJSON}
          onChange={(event) => update({ paramsJSON: event.target.value })}
        />
      </label>
      <div className="flex gap-2 md:col-span-4">
        <Button aria-label={props.value.id > 0 ? "保存任务" : "创建任务"} htmlType="submit" loading={props.actionPending === "save-job"}>
          {props.value.id > 0 ? "保存任务" : "创建任务"}
        </Button>
        <Button aria-label="重置" htmlType="button" onClick={props.onReset}>
          重置
        </Button>
      </div>
    </form>
  );
}
