import { Button, Tag, Typography } from "antd";
import type { SchedulerJob, SchedulerRun } from "../../services/scheduler";

export type SchedulerRunFilters = {
  jobId: string;
  status: string;
  triggerType: string;
};

type SchedulerRunListProps = {
  jobs: SchedulerJob[];
  runs: SchedulerRun[];
  filters: SchedulerRunFilters;
  selectedRun: SchedulerRun | null;
  actionPending: string | null;
  onChangeJob: (jobId: string) => void;
  onChangeStatus: (status: string) => void;
  onChangeTriggerType: (triggerType: string) => void;
  onViewDetail: (run: SchedulerRun) => void;
};

/// SchedulerRunList 展示调度执行记录和详情入口；详情读取仍由页面层通过固定 command 完成。
export function SchedulerRunList(props: SchedulerRunListProps) {
  const visibleRuns = props.runs.filter((run) => {
    const statusMatched = !props.filters.status || run.status === props.filters.status;
    const triggerMatched = !props.filters.triggerType || run.trigger_type === props.filters.triggerType;
    return statusMatched && triggerMatched;
  });
  const runStatuses = uniqueValues(props.runs.map((run) => run.status));
  const triggerTypes = uniqueValues(props.runs.map((run) => run.trigger_type));

  return (
    <>
      <div className="flex flex-col gap-3 border-b border-slate-200 px-4 py-3 md:flex-row md:items-end md:justify-between">
        <Typography.Title level={4} className="m-0">
          最近执行
        </Typography.Title>
        <div className="flex flex-wrap gap-2">
          <select aria-label="执行任务" className="h-8 rounded border border-slate-300 px-2 text-sm" value={props.filters.jobId} onChange={(event) => props.onChangeJob(event.target.value)}>
            <option value="0">全部任务</option>
            {props.jobs.map((job) => (
              <option key={job.id} value={job.id}>
                {job.name || `任务 ${job.id}`}
              </option>
            ))}
          </select>
          <select aria-label="执行状态" className="h-8 rounded border border-slate-300 px-2 text-sm" value={props.filters.status} onChange={(event) => props.onChangeStatus(event.target.value)}>
            <option value="">全部状态</option>
            {runStatuses.map((status) => (
              <option key={status} value={status}>
                {status}
              </option>
            ))}
          </select>
          <select
            aria-label="触发类型"
            className="h-8 rounded border border-slate-300 px-2 text-sm"
            value={props.filters.triggerType}
            onChange={(event) => props.onChangeTriggerType(event.target.value)}
          >
            <option value="">全部触发</option>
            {triggerTypes.map((triggerType) => (
              <option key={triggerType} value={triggerType}>
                {triggerType}
              </option>
            ))}
          </select>
        </div>
      </div>
      {visibleRuns.length === 0 ? (
        <div className="px-4 py-8 text-sm text-slate-500">暂无执行记录</div>
      ) : (
        <div className="grid gap-0">
          {visibleRuns.map((run) => (
            <div key={run.id ?? run.run_key} className="grid gap-2 border-t border-slate-100 px-4 py-3 text-sm md:grid-cols-[1fr_auto_auto_auto_auto_auto_auto_auto]">
              <span className="truncate">{run.run_key}</span>
              <span>{run.trigger_type}</span>
              <span>{run.target_date || "-"}</span>
              <span>{run.started_at || "-"}</span>
              <span>{run.finished_at || "-"}</span>
              <span>
                {run.fetched_count ?? 0}/{run.written_count ?? 0}
              </span>
              <Tag>{run.status}</Tag>
              <Button size="small" aria-label="查看详情" loading={props.actionPending === `run-detail-${run.id}`} onClick={() => props.onViewDetail(run)}>
                详情
              </Button>
              {run.error_message || run.skipped_reason ? (
                <span className="truncate text-xs text-slate-500 md:col-span-8">{run.error_message || run.skipped_reason}</span>
              ) : null}
            </div>
          ))}
        </div>
      )}
      {props.selectedRun ? (
        <div className="border-t border-slate-200 bg-slate-50 px-4 py-4 text-sm">
          <Typography.Title level={5} className="m-0">
            执行详情
          </Typography.Title>
          <div className="mt-3 grid gap-2 md:grid-cols-3">
            <SchedulerRunDetailItem label="Run Key" value={props.selectedRun.run_key} />
            <SchedulerRunDetailItem label="状态" value={props.selectedRun.status} />
            <SchedulerRunDetailItem label="触发" value={props.selectedRun.trigger_type} />
            <SchedulerRunDetailItem label="来源" value={props.selectedRun.source || "-"} />
            <SchedulerRunDetailItem label="目标日期" value={props.selectedRun.target_date || "-"} />
            <SchedulerRunDetailItem label="开始时间" value={props.selectedRun.started_at || "-"} />
            <SchedulerRunDetailItem label="结束时间" value={props.selectedRun.finished_at || "-"} />
            <SchedulerRunDetailItem label="Scope" value={props.selectedRun.scope_key || "-"} />
            <SchedulerRunDetailItem label="类型" value={props.selectedRun.cron_type || "-"} />
            <SchedulerRunDetailItem label="数据" value={props.selectedRun.data_type || "-"} />
            <SchedulerRunDetailItem label="周期" value={props.selectedRun.period || "-"} />
            <SchedulerRunDetailItem label="抓取/写入" value={`${props.selectedRun.fetched_count ?? 0}/${props.selectedRun.written_count ?? 0}`} />
            <SchedulerRunDetailItem label="跳过原因" value={props.selectedRun.skipped_reason || "-"} />
            <SchedulerRunDetailItem label="错误信息" value={props.selectedRun.error_message || "-"} />
          </div>
        </div>
      ) : null}
    </>
  );
}

function SchedulerRunDetailItem(props: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <div className="text-xs text-slate-500">{props.label}</div>
      <div className="truncate font-mono text-xs text-slate-900" title={props.value}>
        {props.value}
      </div>
    </div>
  );
}

function uniqueValues(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort();
}
